package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

const retainSrc = `
type Tree = Leaf(v: Int) | Node(kids: FixedArray[Tree, 2])

type Named = {
    pub name String
    pub mut n Int
}

fn sum(t: Tree) -> Int {
    match t {
        Leaf(v) -> v
        Node(kids) -> {
            let mut s = 0
            for k in kids {
                s = s + sum(k)
            }
            s
        }
    }
}

fn readN(a: Named) -> Int {
    a.n
}

fn keepN(a: Named) -> Named {
    a
}

fn reads(w: Named) -> Int {
    let mut v = 0
    for i in 0..2 {
        v = v + readN(w)
    }
    v
}

fn keeps(n: Named) -> Int {
    let a = n
    let mut b = keepN(a)
    b.n = 9
    a.n + b.n
}

fn writesLoop(xs: FixedArray[Named, 2]) -> Int {
    let mut ys = xs
    let mut total = 0
    for y in ys {
        ys[0].n = ys[0].n + 1
        total = total + y.n
    }
    total + ys[0].n
}

fn main() -> Void! {}
`

func retainsOf(p *mir.Program, f *midir.Func) (retains, copies int) {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpShare {
				continue
			}
			switch {
			case p.InstAllocates(f, in):
				copies++
			case !in.Copy:
				retains++
			}
		}
	}
	return
}

func TestPlanRetainsWhatNothingWrites(t *testing.T) {
	p := planned(t, retainSrc)
	sum := funcNamed(t, p, "main.sum")
	for _, blk := range sum.Blocks {
		for _, in := range blk.Insts {
			if p.InstAllocates(sum, in) {
				t.Errorf("sum still plans an allocating instruction %v:\n%s", in.Op, p.Dump())
			}
		}
	}
	if r, _ := retainsOf(p, sum); r < 2 {
		t.Errorf("sum retains %d times, want the loop's iterable and the recursive argument:\n%s", r, p.Dump())
	}
}

func TestPlanRetainsAnArgumentTheCalleeOnlyReads(t *testing.T) {
	p := planned(t, retainSrc)
	reads := funcNamed(t, p, "main.reads")
	if r, c := retainsOf(p, reads); r == 0 || c != 0 {
		t.Errorf("reads retains %d and copies %d times, want only retains:\n%s", r, c, p.Dump())
	}
}

func TestPlanCopiesWhatACalleeKeeps(t *testing.T) {
	p := planned(t, retainSrc)
	copies := 0
	for _, name := range []string{"main.keeps", "main.keepN"} {
		_, c := retainsOf(p, funcNamed(t, p, name))
		copies += c
	}
	if copies == 0 {
		t.Errorf("keeps writes the box keepN returns without a copy on either side:\n%s", p.Dump())
	}
}

func TestPlanCopiesALoopIterableTheBodyWrites(t *testing.T) {
	p := planned(t, retainSrc)
	f := funcNamed(t, p, "main.writesLoop")
	if _, c := retainsOf(p, f); c == 0 {
		t.Errorf("writesLoop iterates the array it writes without a copy:\n%s", p.Dump())
	}
}
