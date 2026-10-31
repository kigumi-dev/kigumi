package mir_test

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

const asyncFrameSrc = `
type Point = {
    pub mut x Int
    pub mut y Int
}

type Shape = Circle(r: Int) | Rect(w: Int, h: Int)

async fn step(n: Int) -> Int {
    n + 1
}

async fn walk(n: Int) -> Int {
    let mut p = Point { x: n, y: 0 }
    let mut s = Rect(2, 3)
    let grid = FixedArray.filled[u8, 5](1)
    let label = "walk"
    p.y = await step(p.x)
    s = match s {
        Circle(r) -> Rect(r, p.y)
        Rect(w, h) -> Circle(w + h)
    }
    print "${label} ${p.x} ${grid[4]}"
    p.x + p.y
}

fn main() -> Void! {}
`

func localNamed(t *testing.T, f *midir.Func, name string) midir.LocalID {
	t.Helper()
	for i, l := range f.Locals {
		if l.Name == name && l.Ent != 0 {
			return midir.LocalID(i)
		}
	}
	t.Fatalf("%s has no local %s", f.Name, name)
	return 0
}

func TestPlanDirectKeepsAnAsyncLocalInline(t *testing.T) {
	prog := planned(t, asyncFrameSrc)
	f := funcNamed(t, prog, "main.walk")
	want := map[string]int64{"p": 16, "s": 24, "grid": 8, "label": 8}
	type span struct{ at, end int }
	var spans []span
	for name, bytes := range want {
		l := localNamed(t, f, name)
		if f.Slot[l] < 0 {
			t.Fatalf("%s is not in the frame", name)
		}
		if direct := name != "label"; f.IsDirect(l) != direct {
			t.Fatalf("%s: direct=%v, want %v", name, f.IsDirect(l), direct)
		}
		if got := prog.SlotBytes(f, l); got != bytes {
			t.Fatalf("%s takes %d frame bytes, want %d", name, got, bytes)
		}
	}
	for i := range f.Locals {
		if f.Slot[i] >= 0 {
			spans = append(spans, span{f.Slot[i], f.Slot[i] + int(prog.SlotBytes(f, midir.LocalID(i))/8)})
		}
	}
	slices.SortFunc(spans, func(a, b span) int { return a.at - b.at })
	for i := 1; i < len(spans); i++ {
		if spans[i].at < spans[i-1].end {
			t.Fatalf("frame slots overlap: %v", spans)
		}
	}
	if end := spans[len(spans)-1].end; end != f.Frame {
		t.Fatalf("frame is %d words, the slots end at %d", f.Frame, end)
	}
	if n := countOp(f, midir.OpBoxDirect); n != 0 {
		t.Fatalf("walk boxes %d times, want none\n%s", n, prog.Dump())
	}
	if got, want := mir.AsyncFrameBytes(prog.R, f), uint64(32+(f.Frame+len(f.Params))*8); got != want {
		t.Fatalf("frame of %d bytes, want %d", got, want)
	}
}

func TestAggLayoutMatchesTheNativeLayout(t *testing.T) {
	prog := planned(t, `
type Flags = {
    pub on Bool
    pub off Bool
    pub c Char
}

type Mixed = {
    pub on Bool
    pub n Int
}

type Shape = Circle(r: Int) | Rect(w: Int, h: Int) | Dot

type Wide = {
    pub on Bool
    pub v u128
}

type WideShape = Big(v: u128) | Dot

fn use(f: Flags, m: Mixed, s: Shape, b: FixedArray[u8, 5], o: Int?, w: Wide, ws: WideShape, wa: FixedArray[i128, 3]) -> Int { 0 }

fn main() -> Void! {}
`)
	f := funcNamed(t, prog, "main.use")
	want := [][2]int64{{8, 4}, {16, 8}, {24, 8}, {5, 1}, {16, 8}, {24, 8}, {24, 8}, {48, 8}}
	for i, prm := range f.Params {
		ty := f.Locals[prm].Type
		if !prog.R.DirectAgg(ty) {
			t.Fatalf("%s is not direct", prog.R.TypeString(ty))
		}
		size, align := mir.AggLayout(prog.R, ty)
		if size != want[i][0] || align != want[i][1] {
			t.Errorf("%s lays out as %d/%d, want %d/%d", prog.R.TypeString(ty), size, align, want[i][0], want[i][1])
		}
	}
}
