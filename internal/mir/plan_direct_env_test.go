package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

const closureEnvSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn apply(f: fn(Int) -> Int) -> Int { f(1) }

fn local(n: Int, p: Point) -> Int {
    let f = (x: Int) => x + n + p.y
    f(2)
}

fn erases(n: Int) -> Int {
    apply((x: Int) => x + n)
}

fn lends(p: Point) -> Bool {
    let f = () => hash(&p) == hash(&p)
    f()
}

fn main() -> Void! {}
`

func fixedCalls(p *mir.Program, f *mir.Func) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpCall && p.R.Fn(in.Ent).Owner == p.R.LangItem("FixedArray") {
				n++
			}
		}
	}
	return n
}

func firstOp(f *mir.Func, op mir.Opcode) (mir.Inst, bool) {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op {
				return in, true
			}
		}
	}
	return mir.Inst{}, false
}

func TestPlanDirectKeepsAClosureInline(t *testing.T) {
	p := planned(t, closureEnvSrc)
	f := funcNamed(t, p, "main.local")
	clo, ok := firstOp(f, mir.OpClosure)
	if !ok || !f.IsDirect(clo.Dst) {
		t.Fatalf("the closure is not built unboxed:\n%s", p.Dump())
	}
	call, ok := firstOp(f, mir.OpCallValue)
	if !ok || !f.IsDirect(call.Args[0]) {
		t.Errorf("the call does not reach the closure unboxed:\n%s", p.Dump())
	}
	if n := countOp(f, mir.OpBoxDirect); n != 0 {
		t.Errorf("building and calling the closure boxes %d times", n)
	}
	body := funcNamed(t, p, "main.local.closure1")
	if !p.EnvABI(body) || body.DSig.Param(1) == 0 {
		t.Errorf("the body does not read its Point capture out of the environment")
	}
}

func TestPlanDirectBoxesAClosureAtErasure(t *testing.T) {
	p := planned(t, closureEnvSrc)
	f := funcNamed(t, p, "main.erases")
	box, ok := firstOp(f, mir.OpBoxDirect)
	if !ok || p.R.Types.Kind(box.Type) != sem.KClosure || countOp(f, mir.OpBoxDirect) != 1 {
		t.Errorf("want exactly one closure box at the call:\n%s", p.Dump())
	}
}

func TestPlanDirectBoxesALentCaptureOnEntry(t *testing.T) {
	p := planned(t, closureEnvSrc)
	body := funcNamed(t, p, "main.lends.closure1")
	first := body.Blocks[0].Insts[0]
	if first.Op != mir.OpBoxDirect || first.Args[0] != body.Params[0] || !body.IsDirect(body.Params[0]) {
		t.Errorf("the lent capture is not boxed on entry:\n%s", p.Dump())
	}
	if n := countOp(funcNamed(t, p, "main.lends"), mir.OpBoxDirect); n != 0 {
		t.Errorf("the creator boxes %d times", n)
	}
}

const fixedSrc = `
type Point = {
    pub mut x Int
    pub y Int
}

fn inPlace(seed: Int) -> Int {
    let mut fa = FixedArray.filled[Int, 4](seed)
    fa[1] = fa[0] + 1
    let _ = fa.withMut(2, (v: &mut Int) => { v = v * 3 })
    let mut s = 0
    for x in fa {
        s = s + x
    }
    s + (fa.len().toInt() || 0)
}

fn viewWrite() -> Int {
    let mut pts = FixedArray.filled[Point, 2](Point { x: 1, y: 2 })
    pts[0].x = 5
    pts[0].x
}

fn main() -> Void! {}
`

func TestPlanDirectKeepsAFixedArrayInline(t *testing.T) {
	p := planned(t, fixedSrc)
	f := funcNamed(t, p, "main.inPlace")
	if n := countOp(f, mir.OpBoxDirect) + fixedCalls(p, f); n != 0 {
		t.Errorf("the array crosses or is called into %d times:\n%s", n, p.Dump())
	}
	for _, op := range []mir.Opcode{mir.OpIndex, mir.OpSetIndex} {
		in, ok := firstOp(f, op)
		if !ok || !f.IsDirect(in.Args[0]) {
			t.Errorf("%s does not reach the array in place", op)
		}
	}
}

func TestPlanDirectBoxesAnArrayWrittenThroughAView(t *testing.T) {
	p := planned(t, fixedSrc)
	f := funcNamed(t, p, "main.viewWrite")
	set, ok := firstOp(f, mir.OpSetField)
	if !ok || f.IsDirect(set.Args[0]) {
		t.Fatalf("the element view is not boxed:\n%s", p.Dump())
	}
	idx, ok := firstOp(f, mir.OpIndex)
	if !ok || f.IsDirect(idx.Args[0]) {
		t.Errorf("the array the view writes through is not boxed:\n%s", p.Dump())
	}
}
