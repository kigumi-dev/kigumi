package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// program checks a tiny script so the hand-built functions below have a
// type table and a Result to verify against.
func program(t *testing.T) *mir.Program {
	t.Helper()
	res := checkSourceMir(t, "print \"x\"\n")
	return &mir.Program{R: res, ByEnt: map[sem.EntityID]*midir.Func{}}
}

func checkSourceMir(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../../std"), main), IsrContracts: opdef.IsrContracts}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	return res
}

// resource is a move-only type: the Error interface boxed.
func resource(p *mir.Program) sem.TypeID {
	return p.R.Types.Iface(p.R.Types.ErrorEnt(), nil)
}

func ret(args ...midir.LocalID) midir.Term { return midir.Term{Op: midir.TermReturn, Args: args} }

func fn(name string, ret sem.TypeID, locals []midir.Local, blocks ...midir.Block) *midir.Func {
	return &midir.Func{Name: name, Ret: ret, Locals: locals, Blocks: blocks}
}

// expectRule verifies one function and wants the named rule to fire.
func expectRule(t *testing.T, p *mir.Program, f *midir.Func, rule, fragment string) {
	t.Helper()
	p.Funcs = []*midir.Func{f}
	err := p.Verify()
	if err == nil {
		t.Fatalf("%s: verified, want a %s error", f.Name, rule)
	}
	ve, ok := err.(*mir.Error)
	if !ok || ve.Rule != rule || !strings.Contains(ve.Msg, fragment) {
		t.Fatalf("%s: got %v, want rule %s mentioning %q", f.Name, err, rule, fragment)
	}
}

func TestVerifyStructure(t *testing.T) {
	p := program(t)
	unit := []midir.Local{{Name: "t0", Type: sem.TyUnit}}
	expectRule(t, p, fn("noterm", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}}), "structure", "missing terminator")
	expectRule(t, p, fn("range", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpCopy, Dst: 0, Args: []midir.LocalID{7}}}, Term: ret(0)}), "structure", "out of range")
	expectRule(t, p, fn("target", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{3}}}), "structure", "target b3 out of range")
	expectRule(t, p, fn("branch", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{0}, Targets: []midir.BlockID{0}}}), "structure", "two targets")
}

func TestVerifyShape(t *testing.T) {
	p := program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyI64}}
	expectRule(t, p, fn("arity", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpBinary, Dst: 0, Str: "+", Args: []midir.LocalID{1}}}, Term: ret(0)}), "shape", "takes 2 arguments")
	expectRule(t, p, fn("notype", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0}}, Term: ret(0)}), "shape", "needs a type")
	expectRule(t, p, fn("noent", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpFnItem, Dst: 0}}, Term: ret(0)}), "shape", "needs an entity")
	expectRule(t, p, fn("holes", sem.TyString, []midir.Local{{Name: "t0", Type: sem.TyString}, {Name: "t1", Type: sem.TyI64}}, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}, {Op: midir.OpInterp, Dst: 0, Strs: []string{"a"}, Args: []midir.LocalID{1}}}, Term: ret(0)}), "shape", "holes")
}

func TestVerifyTypes(t *testing.T) {
	p := program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyBool}, {Name: "t2", Type: sem.TyString}}
	boolConst := midir.Inst{Op: midir.OpConst, Dst: 1, Type: sem.TyBool, Lit: sem.Literal{Kind: sem.LitBool}}
	expectRule(t, p, fn("copy", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{boolConst, {Op: midir.OpCopy, Dst: 0, Args: []midir.LocalID{1}}}, Term: ret(0)}), "types", "copy from Bool into Int")
	expectRule(t, p, fn("const", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyBool, Lit: sem.Literal{Kind: sem.LitBool}}}, Term: ret(0)}), "types", "constant of Bool into Int")
	expectRule(t, p, fn("cond", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}}, Term: midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{0}, Targets: []midir.BlockID{1, 1}}}, midir.Block{Term: ret(0)}), "types", "branch on Int")
	expectRule(t, p, fn("ret", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{boolConst}, Term: ret(1)}), "types", "return of Bool")
	expectRule(t, p, fn("interp", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpInterp, Dst: 0, Strs: []string{"x"}}}, Term: ret(0)}), "types", "yields String")
	// A bare Void only stands for Ok(()) on the ImplicitOk script exit
	// (build.go); an ordinary fallible-Void function must return an Ok.
	unitRet := p.R.Types.Result(sem.TyUnit, resource(p))
	unitLocals := []midir.Local{{Name: "t0", Type: sem.TyUnit}}
	expectRule(t, p, fn("bareunit", unitRet, unitLocals, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: ret(0)}), "types", "return of Void")
	implicitOk := fn("main", unitRet, unitLocals, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}, Term: ret(0)})
	implicitOk.ImplicitOk = true
	p.Funcs = []*midir.Func{implicitOk}
	if err := p.Verify(); err != nil {
		t.Fatalf("ImplicitOk bare Void return: got %v, want no error", err)
	}
}

// TestVerifyFnPurityErasure checks that a copy from a plain fn value into a
// `pure`/`noalloc`-qualified fn-typed slot (or a plain fn item)
// verifies, since purity is the effects pass's job, not the runtime
// representation's.
func TestVerifyFnPurityErasure(t *testing.T) {
	p := program(t)
	tt := p.R.Types
	plain := tt.Fn([]sem.TypeID{sem.TyI64}, sem.TyI64, 0, false)
	pure := tt.Fn([]sem.TypeID{sem.TyI64}, sem.TyI64, sem.EffPure, false)
	locals := []midir.Local{{Name: "t0", Type: pure}, {Name: "t1", Type: plain}}
	f := fn("erase", pure, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpCopy, Dst: 0, Args: []midir.LocalID{1}}}, Term: ret(0)})
	f.Params = []midir.LocalID{1}
	p.Funcs = []*midir.Func{f}
	if err := p.Verify(); err != nil {
		t.Fatalf("erase: got %v, want no error", err)
	}
}

func TestVerifyDefs(t *testing.T) {
	p := program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyBool}, {Name: "t1", Type: sem.TyI64}, {Name: "t2", Type: sem.TyI64}}
	// t1 is assigned on one branch only, then read at the join.
	f := fn("halfdef", sem.TyI64, locals,
		midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyBool, Lit: sem.Literal{Kind: sem.LitBool}}}, Term: midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{0}, Targets: []midir.BlockID{1, 2}}},
		midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{2}}},
		midir.Block{Insts: []midir.Inst{{Op: midir.OpCopy, Dst: 2, Args: []midir.LocalID{1}}}, Term: ret(2)})
	expectRule(t, p, f, "defs", "reads %t1 before any definition")
}

func TestVerifyOwnership(t *testing.T) {
	p := program(t)
	res := resource(p)
	ent := p.R.Types.ErrorEnt()
	locals := []midir.Local{{Name: "r", Type: res, Ent: ent}, {Name: "t1", Type: res}, {Name: "t2", Type: sem.TyUnit}}
	unit := midir.Inst{Op: midir.OpUnit, Dst: 2}
	leak := fn("leak", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{unit}, Term: ret(2)})
	leak.Params = []midir.LocalID{0}
	expectRule(t, p, leak, "ownership", "still owned at return")
	twice := fn("twice", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpDrop, Args: []midir.LocalID{0}}, {Op: midir.OpDrop, Args: []midir.LocalID{0}}, unit}, Term: ret(2)})
	twice.Params = []midir.LocalID{0}
	expectRule(t, p, twice, "ownership", "holds nothing")
	after := fn("after", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpMove, Dst: 1, Args: []midir.LocalID{0}}, {Op: midir.OpCopy, Dst: 1, Args: []midir.LocalID{0}}, unit}, Term: ret(2)})
	after.Params = []midir.LocalID{0}
	expectRule(t, p, after, "ownership", "after it was consumed")
	over := fn("over", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpMove, Dst: 1, Args: []midir.LocalID{0}}, {Op: midir.OpMove, Dst: 0, Args: []midir.LocalID{1}}, {Op: midir.OpMove, Dst: 0, Args: []midir.LocalID{1}}, unit}, Term: ret(2)})
	over.Params = []midir.LocalID{0}
	expectRule(t, p, over, "ownership", "still holds a value")
}

func TestVerifyDropOwned(t *testing.T) {
	p := program(t)
	ref := p.R.Types.Ref(resource(p), true)
	locals := []midir.Local{{Name: "b", Type: resource(p), Ent: p.R.Types.ErrorEnt()}, {Name: "t1", Type: ref}, {Name: "t2", Type: sem.TyUnit}}
	unit := midir.Inst{Op: midir.OpUnit, Dst: 2}
	f := fn("droplent", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{
		{Op: midir.OpBorrow, Dst: 1, Args: []midir.LocalID{0}, Index: 1},
		{Op: midir.OpDrop, Args: []midir.LocalID{1}},
		{Op: midir.OpDrop, Args: []midir.LocalID{0}},
		unit,
	}, Term: ret(2)})
	f.Params = []midir.LocalID{0}
	expectRule(t, p, f, "droplent", "a reference this frame does not own")
}

// TestVerifyDropOwnedAfterDirectMutation checks a second Verify call recomputes unownedRefs instead of reusing the first call's cache.
func TestVerifyDropOwnedAfterDirectMutation(t *testing.T) {
	p := program(t)
	ref := p.R.Types.Ref(resource(p), true)
	locals := []midir.Local{{Name: "b", Type: resource(p), Ent: p.R.Types.ErrorEnt()}, {Name: "t1", Type: ref, Ent: p.R.Types.ErrorEnt()}, {Name: "t2", Type: sem.TyUnit}}
	unit := midir.Inst{Op: midir.OpUnit, Dst: 2}
	f := fn("recompute", sem.TyUnit, locals, midir.Block{Insts: []midir.Inst{
		{Op: midir.OpAlias, Dst: 1, Args: []midir.LocalID{0}},
		{Op: midir.OpDrop, Args: []midir.LocalID{1}},
		{Op: midir.OpDrop, Args: []midir.LocalID{0}},
		unit,
	}, Term: ret(2)})
	f.Params = []midir.LocalID{0}
	p.Funcs = []*midir.Func{f}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify before mutation: %v", err)
	}
	f.Blocks[0].Insts[0].Op, f.Blocks[0].Insts[0].Index = midir.OpBorrow, 1
	err := p.Verify()
	ve, ok := err.(*mir.Error)
	if !ok || ve.Rule != "droplent" {
		t.Fatalf("got %v, want a droplent error after the mutation turned t1 into a borrow", err)
	}
}

func TestVerifyCellCaptures(t *testing.T) {
	res := checkSourceMir(t, `
fn bump() -> Int {
    let mut n = 0
    let step = () => { n = n + 1 }
    step()
    n
}
let _ = bump()
`)
	prog := mir.Build(res)
	if err := prog.Verify(); err != nil {
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
	err := prog.Verify()
	ve, ok := err.(*mir.Error)
	if !ok || ve.Rule != "cellcapture" {
		t.Fatalf("got %v, want a cellcapture error", err)
	}
}

// TestVerifyCellCapturesNested checks that a closure forwarding its own
// captured cell to a nested closure verifies: build.go never rewraps a
// forwarded capture, and the trace back to the outer OpNewCell must follow
// it through both layers.
func TestVerifyCellCapturesNested(t *testing.T) {
	res := checkSourceMir(t, `
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
	prog := mir.Build(res)
	if err := prog.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatalf("plan-rc: %v\n%s", err, prog.Dump())
	}
}
