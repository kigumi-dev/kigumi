package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const fieldCopyPrelude = `type Pt = {
    pub mut x Int
    pub mut y Int
}

type Seg = {
    pub mut a Pt
    pub mut b Pt
}

type Wrap = {
    pub mut a FixedArray[Pt, 3]
}

type Outer = {
    pub mut segs FixedArray[Seg, 2]
    pub mut w Wrap
}

fn mk() -> Outer {
    let p = Pt { x: 1, y: 2 }
    Outer { segs: FixedArray.filled[Seg, 2](Seg { a: p, b: p }), w: Wrap { a: FixedArray.filled[Pt, 3](p) } }
}
`

var fieldCopyShapes = []struct{ name, src string }{
	{"param", fieldCopyPrelude + `
noalloc fn op(o: &mut Outer) {
    %s
}

fn run(rounds: Int) -> Int {
    let mut o = mk()
    let mut s = 0
    for i in 0..rounds {
        op(&mut o)
        s = s + i
    }
    s + o.segs[0].a.x
}

print "ok=${run(%d)}"
`},
	{"method", fieldCopyPrelude + `
noalloc fn Outer.op(mut self) {
    %s
}

fn run(rounds: Int) -> Int {
    let mut o = mk()
    let mut s = 0
    for i in 0..rounds {
        o.op()
        s = s + i
    }
    s + o.segs[0].a.x
}

print "ok=${run(%d)}"
`},
}

var fieldCopyBodies = []string{
	"%[1]s.segs[0].a = %[1]s.segs[1].b",
	"%[1]s.w.a[1] = %[1]s.segs[1].a",
	"%[1]s.segs[1].b = %[1]s.w.a[2]",
}

// A copy between record fields, or between a field and a slot, fills the destination's own box like the slot-to-slot form.
func TestRecordFieldCopyAllocatesNothing(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, sh := range fieldCopyShapes {
		for bi, body := range fieldCopyBodies {
			t.Run(fmt.Sprintf("%s%d", sh.name, bi), func(t *testing.T) {
				t.Parallel()
				recv := "o"
				if sh.name == "method" {
					recv = "self"
				}
				copyBody := fmt.Sprintf(body, recv)
				src := func(b string, rounds int) string { return fmt.Sprintf(sh.src, b, rounds) }
				slope := func(count func(*testing.T, string) int, b string) int {
					return count(t, src(b, 6)) - count(t, src(b, 2))
				}
				if got, want := slope(nativeAllocs, copyBody), slope(nativeAllocs, "let k = 0"); got != want {
					t.Errorf("native: the field copy allocates %d over 4 rounds, a body without it %d", got, want)
				}
				if got := slope(vmAllocs, copyBody); got != 0 {
					t.Errorf("vm: the field copy allocates %d over 4 rounds", got)
				}
			})
		}
	}
}

const ownedRootStoreSrc = fieldCopyPrelude + `
fn bySlots(mut w: Wrap, i: usize, j: usize) {
    w.a[i] = w.a[j]
}

fn bySlotsFixed(mut w: Wrap) {
    w.a[0] = w.a[2]
}

fn byFields(mut o: Outer) {
    o.segs[0].a = o.segs[1].b
}

fn bySeg(mut o: Outer) {
    o.segs[0] = o.segs[1]
}

fn bySwap(mut w: Wrap) {
    let t = w.a[0]
    w.a[1] = t
    w.a[2] = Pt { x: 1, y: 2 }
}

fn local(p: Pt) {
    let mut w = Wrap { a: FixedArray.filled[Pt, 3](p) }
    w.a[0] = w.a[2]
    w.a[1] = Pt { x: 3, y: 4 }
}

fn localFields(p: Pt) {
    let mut o = mk()
    o.segs[0].a = o.segs[1].b
    o.w.a[1] = o.segs[1].a
}

type Tw = {
    pub mut ws Array[Int]
    pub mut w Wrap
}

fn mkTw() -> Tw {
    Tw { ws: Array.of(1), w: mk().w }
}

fn twLiteral(mut t: Tw) -> Int {
    t.w.a[0] = Pt { x: 1, y: 2 }
    t.w.a[0].x
}

fn twSwap(mut t: Tw) -> Int {
    let tmp = t.w.a[0]
    t.w.a[1] = tmp
    t.w.a[1].x
}

fn twOwn(mut t: Tw) -> Tw {
    t.w.a[2] = Pt { x: 9, y: 9 }
    t
}

fn twCopy(mut t: Tw) -> Int {
    t.w.a[0] = t.w.a[1]
    t.w.a[0].x
}

fn caller() {
    let o = mk()
    let _ = twLiteral(mkTw())
    let _ = twSwap(mkTw())
    let _ = twOwn(mkTw())
    let _ = twCopy(mkTw())
    bySlots(o.w, 0, 1)
    bySlotsFixed(o.w)
    byFields(o)
    bySeg(o)
    bySwap(o.w)
    local(Pt { x: 1, y: 2 })
    localFields(Pt { x: 1, y: 2 })
}

caller()
`

// A slot or field store through a root the body owns is charged or done in place, never left for the alloc-claim check to find.
func TestOwnedRootStoresNeverReachTheAllocClaim(t *testing.T) {
	t.Parallel()
	a, err := driver.Analyze(loadTryConstructorModule(t, ownedRootStoreSrc))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("unexpected diagnostics:\n%s", a.Res.Render())
	}
}
