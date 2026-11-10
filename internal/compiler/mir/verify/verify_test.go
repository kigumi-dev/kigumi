package verify_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

// expectRule verifies one function and wants the named rule to fire.
func expectRule(t *testing.T, p *mir.Program, f *midir.Func, rule, fragment string) {
	t.Helper()
	p.Funcs = []*midir.Func{f}
	err := mirverify.Program(p)
	if err == nil {
		t.Fatalf("%s: verified, want a %s error", f.Name, rule)
	}
	ve, ok := err.(*mirverify.Error)
	if !ok || ve.Rule != rule || !strings.Contains(ve.Msg, fragment) {
		t.Fatalf("%s: got %v, want rule %s mentioning %q", f.Name, err, rule, fragment)
	}
}

func TestVerifyStructure(t *testing.T) {
	p := mirfixture.Program(t)
	unit := []midir.Local{{Name: "t0", Type: sem.TyUnit}}
	expectRule(t, p, mirfixture.Fn("noterm", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}}), "structure", "missing terminator")
	expectRule(t, p, mirfixture.Fn("range", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpCopy, Dst: 0, Args: []midir.LocalID{7}}}, Term: mirfixture.Ret(0)}), "structure", "out of range")
	expectRule(t, p, mirfixture.Fn("target", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{3}}}), "structure", "target b3 out of range")
	expectRule(t, p, mirfixture.Fn("branch", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{0}, Targets: []midir.BlockID{0}}}), "structure", "two targets")
}

func TestVerifyShape(t *testing.T) {
	p := mirfixture.Program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyI64}}
	expectRule(t, p, mirfixture.Fn("arity", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpBinary, Dst: 0, Str: "+", Args: []midir.LocalID{1}}}, Term: mirfixture.Ret(0)}), "shape", "takes 2 arguments")
	expectRule(t, p, mirfixture.Fn("notype", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0}}, Term: mirfixture.Ret(0)}), "shape", "needs a type")
	expectRule(t, p, mirfixture.Fn("noent", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpFnItem, Dst: 0}}, Term: mirfixture.Ret(0)}), "shape", "needs an entity")
	expectRule(t, p, mirfixture.Fn("holes", sem.TyString, []midir.Local{{Name: "t0", Type: sem.TyString}, {Name: "t1", Type: sem.TyI64}}, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}, {Op: midir.OpInterp, Dst: 0, Strs: []string{"a"}, Args: []midir.LocalID{1}}}, Term: mirfixture.Ret(0)}), "shape", "holes")
}

func TestVerifyTypes(t *testing.T) {
	p := mirfixture.Program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyBool}, {Name: "t2", Type: sem.TyString}}
	boolConst := midir.Inst{Op: midir.OpConst, Dst: 1, Type: sem.TyBool, Lit: sem.Literal{Kind: sem.LitBool}}
	expectRule(t, p, mirfixture.Fn("copy", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{boolConst, {Op: midir.OpCopy, Dst: 0, Args: []midir.LocalID{1}}}, Term: mirfixture.Ret(0)}), "types", "copy from Bool into Int")
	expectRule(t, p, mirfixture.Fn("const", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyBool, Lit: sem.Literal{Kind: sem.LitBool}}}, Term: mirfixture.Ret(0)}), "types", "constant of Bool into Int")
	expectRule(t, p, mirfixture.Fn("cond", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}}, Term: midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{0}, Targets: []midir.BlockID{1, 1}}}, midir.Block{Term: mirfixture.Ret(0)}), "types", "branch on Int")
	expectRule(t, p, mirfixture.Fn("ret", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{boolConst}, Term: mirfixture.Ret(1)}), "types", "return of Bool")
	expectRule(t, p, mirfixture.Fn("interp", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpInterp, Dst: 0, Strs: []string{"x"}}}, Term: mirfixture.Ret(0)}), "types", "yields String")
	// A bare Void only stands for Ok(()) on the ImplicitOk script exit
	// (build.go); an ordinary fallible-Void function must return an Ok.
	unitRet := p.R.Types.Result(sem.TyUnit, mirfixture.Resource(p))
	unitLocals := []midir.Local{{Name: "t0", Type: sem.TyUnit}}
	expectRule(t, p, mirfixture.Fn("bareunit", unitRet, unitLocals, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: mirfixture.Ret(0)}), "types", "return of Void")
	implicitOk := mirfixture.Fn("main", unitRet, unitLocals, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: mirfixture.Ret(0)})
	implicitOk.ImplicitOk = true
	p.Funcs = []*midir.Func{implicitOk}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("ImplicitOk bare Void return: got %v, want no error", err)
	}
}

// Purity is the effects pass's job, so copying a plain fn value into a `pure`/`noalloc` fn slot verifies.
func TestVerifyFnPurityErasure(t *testing.T) {
	p := mirfixture.Program(t)
	tt := p.R.Types
	plain := tt.Fn([]sem.TypeID{sem.TyI64}, sem.TyI64, 0, false)
	pure := tt.Fn([]sem.TypeID{sem.TyI64}, sem.TyI64, sem.EffPure, false)
	locals := []midir.Local{{Name: "t0", Type: pure}, {Name: "t1", Type: plain}}
	f := mirfixture.Fn("erase", pure, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpCopy, Dst: 0, Args: []midir.LocalID{1}}}, Term: mirfixture.Ret(0)})
	f.Params = []midir.LocalID{1}
	p.Funcs = []*midir.Func{f}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("erase: got %v, want no error", err)
	}
}

func TestVerifyDefs(t *testing.T) {
	p := mirfixture.Program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyBool}, {Name: "t1", Type: sem.TyI64}, {Name: "t2", Type: sem.TyI64}}
	// t1 is assigned on one branch only, then read at the join.
	f := mirfixture.Fn("halfdef", sem.TyI64, locals,
		midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyBool, Lit: sem.Literal{Kind: sem.LitBool}}}, Term: midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{0}, Targets: []midir.BlockID{1, 2}}},
		midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{2}}},
		midir.Block{Insts: []midir.Inst{{Op: midir.OpCopy, Dst: 2, Args: []midir.LocalID{1}}}, Term: mirfixture.Ret(2)})
	expectRule(t, p, f, "defs", "reads %t1 before any definition")
}

// A block nothing targets was never attached to the CFG: nothing defines any
// local for it, so a read there has to define its own operand first.
func TestVerifyRejectsUndefInUnreachableBlock(t *testing.T) {
	p := mirfixture.Program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "undef", Type: sem.TyI64}}
	f := mirfixture.Fn("dead", sem.TyI64, locals,
		midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}}, Term: mirfixture.Ret(0)},
		midir.Block{Insts: []midir.Inst{{Op: midir.OpMove, Dst: 0, Args: []midir.LocalID{1}}}, Term: mirfixture.Ret(0)})
	expectRule(t, p, f, "defs", "reads %undef before any definition")
}

func TestVerifyOwnership(t *testing.T) {
	p := mirfixture.Program(t)
	res := mirfixture.Resource(p)
	ent := p.R.Types.ErrorEnt()
	locals := []midir.Local{{Name: "r", Type: res, Ent: ent}, {Name: "t1", Type: res}, {Name: "t2", Type: sem.TyUnit}}
	unit := midir.Inst{Op: midir.OpUnit, Dst: 2}
	leak := mirfixture.Fn("leak", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{unit}, Term: mirfixture.Ret(2)})
	leak.Params = []midir.LocalID{0}
	expectRule(t, p, leak, "ownership", "still owned at return")
	twice := mirfixture.Fn("twice", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpDrop, Args: []midir.LocalID{0}}, {Op: midir.OpDrop, Args: []midir.LocalID{0}}, unit}, Term: mirfixture.Ret(2)})
	twice.Params = []midir.LocalID{0}
	expectRule(t, p, twice, "ownership", "holds nothing")
	after := mirfixture.Fn("after", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpMove, Dst: 1, Args: []midir.LocalID{0}}, {Op: midir.OpCopy, Dst: 1, Args: []midir.LocalID{0}}, unit}, Term: mirfixture.Ret(2)})
	after.Params = []midir.LocalID{0}
	expectRule(t, p, after, "ownership", "after it was consumed")
	over := mirfixture.Fn("over", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpMove, Dst: 1, Args: []midir.LocalID{0}}, {Op: midir.OpMove, Dst: 0, Args: []midir.LocalID{1}}, {Op: midir.OpMove, Dst: 0, Args: []midir.LocalID{1}}, unit}, Term: mirfixture.Ret(2)})
	over.Params = []midir.LocalID{0}
	expectRule(t, p, over, "ownership", "still holds a value")
	unreachable := mirfixture.Fn("unreachable", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{unit}, Term: midir.Term{Op: midir.TermUnreachable}})
	unreachable.Params = []midir.LocalID{0}
	expectRule(t, p, unreachable, "ownership", "still owned at unreachable")
}

func TestVerifyDropOwned(t *testing.T) {
	p := mirfixture.Program(t)
	ref := p.R.Types.Ref(mirfixture.Resource(p), true)
	locals := []midir.Local{{Name: "b", Type: mirfixture.Resource(p), Ent: p.R.Types.ErrorEnt()}, {Name: "t1", Type: ref}, {Name: "t2", Type: sem.TyUnit}}
	unit := midir.Inst{Op: midir.OpUnit, Dst: 2}
	f := mirfixture.Fn("droplent", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{
		{Op: midir.OpBorrow, Dst: 1, Args: []midir.LocalID{0}, Index: 1},
		{Op: midir.OpDrop, Args: []midir.LocalID{1}},
		{Op: midir.OpDrop, Args: []midir.LocalID{0}},
		unit,
	}, Term: mirfixture.Ret(2)})
	f.Params = []midir.LocalID{0}
	expectRule(t, p, f, "droplent", "a reference this frame does not own")
}

// TestVerifyDropOwnedAfterDirectMutation checks a second Verify call recomputes unownedRefs instead of reusing the first call's cache.
func TestVerifyDropOwnedAfterDirectMutation(t *testing.T) {
	p := mirfixture.Program(t)
	ref := p.R.Types.Ref(mirfixture.Resource(p), true)
	locals := []midir.Local{{Name: "b", Type: mirfixture.Resource(p), Ent: p.R.Types.ErrorEnt()}, {Name: "t1", Type: ref, Ent: p.R.Types.ErrorEnt()}, {Name: "t2", Type: sem.TyUnit}}
	unit := midir.Inst{Op: midir.OpUnit, Dst: 2}
	f := mirfixture.Fn("recompute", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{
		{Op: midir.OpAlias, Dst: 1, Args: []midir.LocalID{0}},
		{Op: midir.OpDrop, Args: []midir.LocalID{1}},
		{Op: midir.OpDrop, Args: []midir.LocalID{0}},
		unit,
	}, Term: mirfixture.Ret(2)})
	f.Params = []midir.LocalID{0}
	p.Funcs = []*midir.Func{f}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("verify before mutation: %v", err)
	}
	f.Blocks[0].Insts[0].Op, f.Blocks[0].Insts[0].Index = midir.OpBorrow, 1
	err := mirverify.Program(p)
	ve, ok := err.(*mirverify.Error)
	if !ok || ve.Rule != "droplent" {
		t.Fatalf("got %v, want a droplent error after the mutation turned t1 into a borrow", err)
	}
}

func TestVerifyCellCaptures(t *testing.T) {
	res := mirfixture.CheckSourceMir(t, `
fn bump() -> Int {
    let mut n = 0
    let step = () => { n = n + 1 }
    step()
    n
}
let _ = bump()
`)
	prog := mirbuild.Program(res)
	if err := mirverify.Program(prog); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var target *midir.Func
	for _, f := range prog.Funcs {
		if strings.HasSuffix(f.Name, ".bump") {
			target = f
		}
	}
	if target == nil {
		t.Fatal("bump not found among the built functions")
	}
	patched := false
	for bi := range target.Blocks {
		for ii := range target.Blocks[bi].Insts {
			if target.Blocks[bi].Insts[ii].Op == midir.OpNewCell {
				target.Blocks[bi].Insts[ii].Op = midir.OpCopy
				patched = true
			}
		}
	}
	if !patched {
		t.Fatal("bump has no OpNewCell to break")
	}
	err := mirverify.Program(prog)
	ve, ok := err.(*mirverify.Error)
	if !ok || ve.Rule != "cellcapture" {
		t.Fatalf("got %v, want a cellcapture error", err)
	}
}

// Lowering never rewraps a forwarded capture, so the trace back to the outer OpNewCell crosses both closures.
func TestVerifyCellCapturesNested(t *testing.T) {
	res := mirfixture.CheckSourceMir(t, `
type Counter = {
    pub mut n Int
}
fn Counter.nested(mut self) {
    let outer = () => {
        let inner = () => { self.n = self.n + 1 }
        inner()
    }
    outer()
}
let mut c = Counter { n: 0 }
c.nested()
`)
	prog := mirbuild.Program(res)
	if err := mirverify.Program(prog); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatalf("plan-rc: %v\n%s", err, prog.Dump())
	}
}
