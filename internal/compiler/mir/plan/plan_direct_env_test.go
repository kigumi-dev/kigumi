package plan_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

func fixedCalls(p *mir.Program, f *midir.Func) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpCall && p.R.Fn(in.Ent).Owner == p.R.LangItem("FixedArray") {
				n++
			}
		}
	}
	return n
}

func TestPlanDirectKeepsAClosureInline(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.ClosureEnvSrc)
	f := mirfixture.FuncNamed(t, p, "main.local")
	clo, ok := mirfixture.FirstOp(f, midir.OpClosure)
	if !ok || !f.IsDirect(clo.Dst) {
		t.Fatalf("the closure is not built unboxed:\n%s", p.Dump())
	}
	call, ok := mirfixture.FirstOp(f, midir.OpCallValue)
	if !ok || !f.IsDirect(call.Args[0]) {
		t.Errorf("the call does not reach the closure unboxed:\n%s", p.Dump())
	}
	if n := mirfixture.CountOp(f, midir.OpBoxDirect); n != 0 {
		t.Errorf("building and calling the closure boxes %d times", n)
	}
	body := mirfixture.FuncNamed(t, p, "main.local.closure1")
	if !p.EnvABI(body) || body.DSig.Param(1) == 0 {
		t.Errorf("the body does not read its Point capture out of the environment")
	}
}

func TestPlanDirectBoxesAClosureAtErasure(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.ClosureEnvSrc)
	f := mirfixture.FuncNamed(t, p, "main.erases")
	box, ok := mirfixture.FirstOp(f, midir.OpBoxDirect)
	if !ok || p.R.Types.Kind(box.Type) != sem.KClosure || mirfixture.CountOp(f, midir.OpBoxDirect) != 1 {
		t.Errorf("want exactly one closure box at the call:\n%s", p.Dump())
	}
}

func TestPlanDirectBoxesALentCaptureOnEntry(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.ClosureEnvSrc)
	body := mirfixture.FuncNamed(t, p, "main.lends.closure1")
	first := body.Blocks[0].Insts[0]
	if first.Op != midir.OpBoxDirect || first.Args[0] != body.Params[0] || !body.IsDirect(body.Params[0]) {
		t.Errorf("the lent capture is not boxed on entry:\n%s", p.Dump())
	}
	if n := mirfixture.CountOp(mirfixture.FuncNamed(t, p, "main.lends"), midir.OpBoxDirect); n != 0 {
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
	p := mirfixture.Planned(t, fixedSrc)
	f := mirfixture.FuncNamed(t, p, "main.inPlace")
	if n := mirfixture.CountOp(f, midir.OpBoxDirect) + fixedCalls(p, f); n != 0 {
		t.Errorf("the array crosses or is called into %d times:\n%s", n, p.Dump())
	}
	for _, op := range []midir.Opcode{midir.OpIndex, midir.OpSetIndex} {
		in, ok := mirfixture.FirstOp(f, op)
		if !ok || !f.IsDirect(in.Args[0]) {
			t.Errorf("%s does not reach the array in place", op)
		}
	}
}

func TestPlanDirectBoxesAnArrayWrittenThroughAView(t *testing.T) {
	p := mirfixture.Planned(t, fixedSrc)
	f := mirfixture.FuncNamed(t, p, "main.viewWrite")
	set, ok := mirfixture.FirstOp(f, midir.OpSetField)
	if !ok || f.IsDirect(set.Args[0]) {
		t.Fatalf("the element view is not boxed:\n%s", p.Dump())
	}
	idx, ok := mirfixture.FirstOp(f, midir.OpIndex)
	if !ok || f.IsDirect(idx.Args[0]) {
		t.Errorf("the array the view writes through is not boxed:\n%s", p.Dump())
	}
}
