package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/devkit/mirfixture"
)

const twiceSrc = `
type Named = {
    pub name String
    pub mut n Int
}

type V = {
    pub mut a Named
    pub mut c Named
}

type Y = {
    pub mut a Named
    pub mut ns Array[Named]
}

fn twice(v: V) -> V {
    V { a: v.a, c: v.a }
}

fn apart(v: V) -> V {
    V { a: v.c, c: v.a }
}

fn carried(y: Y) -> Y {
    Y { a: y.ns[1], ns: move y.ns }
}

fn rebound(mut v: V) -> Int {
    let mut i = 0
    for i < 2 {
        v = V { a: v.c, c: v.a }
        i = i + 1
    }
    v.a.n
}

fn swap(w: &mut V) {
    let t = w.a
    w.a = w.c
    w.c = t
}

fn readBetween(w: &mut V) -> Int {
    let t = w.a
    w.a = w.c
    let k = w.c.n
    w.c = t
    k
}

fn main() -> Void! {}
`

func TestPlanRCCopiesAReadSunkTwice(t *testing.T) {
	prog := rcPlanned(t, twiceSrc)
	for name, want := range map[string]int{
		"main.twice": 1, "main.apart": 0, "main.carried": 1, "main.rebound": 0,
	} {
		if got := len(copies(mirfixture.FuncNamed(t, prog, name))); got != want {
			t.Fatalf("%s: %d deep copies, want %d\n%s", name, got, want, prog.Dump())
		}
	}
}

func TestPlanRCMovesAFieldStoredOver(t *testing.T) {
	prog := rcPlanned(t, twiceSrc)
	for name, want := range map[string]int{"main.swap": 0, "main.readBetween": 1} {
		if got := ownCopies(mirfixture.FuncNamed(t, prog, name)); got != want {
			t.Fatalf("%s: %d deep copies, want %d\n%s", name, got, want, prog.Dump())
		}
	}
}

func ownCopies(f *midir.Func) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpShare && in.Own {
				n++
			}
		}
	}
	return n
}
