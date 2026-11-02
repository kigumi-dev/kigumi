package verify_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const cellRulesSrc = rulesSrc + `
fn Point.bump(mut self) { self.y += 1 }

fn drive() -> Int {
    let mut q = Point { x: 1, y: 2 }
    q.bump()
    q.y
}

let _ = both(Point { x: 1, y: 2 })
`

func TestVerifyCellValues(t *testing.T) {
	runRuleCases(t, cellRulesSrc, false, []ruleCase{
		{"take of a received cell", "main.Point.bump", "cellops", "celltake empties %self, a cell this frame did not make", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCellGet, 0).Op = midir.OpCellTake
		}},
		{"wrap of a received cell", "main.Point.bump", "cellops", "newcell wraps %self, which is already a cell", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCellGet, 0).Op = midir.OpNewCell
		}},
		{"borrow handed to a mut receiver", "main.drive", "cellops", "call hands %t4, which is no cell, to a mut receiver", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCall, 0).Args[0] = nth(t, f, midir.OpBorrow, 0).Dst
		}},
		{"cell stored in itself", "main.Point.bump", "cellops", "cellset reads the cell %self as a plain value", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpCellSet, Args: []midir.LocalID{f.Params[0], f.Params[0]}})
		}},
		{"cell read as a value", "main.Point.bump", "cellops", "field reads the cell %self as a plain value", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpField, 0).Args[0] = f.Params[0]
		}},
	})
}

func TestVerifyFutureRules(t *testing.T) {
	future := func(p *mir.Program, ret sem.TypeID) sem.TypeID {
		return p.R.Types.Named(p.R.Types.FutureEnt(), []sem.TypeID{ret})
	}
	cases := []struct {
		name, fragment string
		edit           func(p *mir.Program, f *midir.Func, in *midir.Inst)
	}{
		{"future of a value", "which is not an async function item", func(p *mir.Program, f *midir.Func, in *midir.Inst) { in.Args[0] = in.Args[1] }},
		{"future of another kind", "yields Int, not a Future", func(p *mir.Program, f *midir.Func, in *midir.Inst) { f.Locals[in.Dst].Type = sem.TyI64 }},
		{"future of another result", "which returns Int", func(p *mir.Program, f *midir.Func, in *midir.Inst) {
			f.Locals[in.Dst].Type = future(p, sem.TyString)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := mirfixture.Planned(t, asyncRulesSrc)
			f := mirfixture.FuncNamed(t, p, "main")
			c.edit(p, f, nth(t, f, midir.OpFutureNew, 0))
			mirfixture.WantRule(t, p, "calls", c.fragment)
		})
	}
}

const declaredSrc = `
extern(C) {
    fn labs(x: i64) -> i64
}

fn absolute(x: i64) -> i64 { unsafe { labs(x) } }
`

func TestVerifyDeclaredCalls(t *testing.T) {
	runRuleCases(t, declaredSrc, false, []ruleCase{
		{"extra argument", "main.absolute", "calls", "passes 2 values to labs, which takes 1", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCall, 0)
			in.Args = append(in.Args, in.Args[0])
		}},
		{"argument of another type", "main.absolute", "calls", "passes String as argument 0 of labs, which takes Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCall, 0).Args[0] = newString(f)
		}},
		{"result of another type", "main.absolute", "calls", "takes String from labs, which returns Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCall, 0).Dst = addLocal(f, "s", sem.TyString)
		}},
	})
}

func TestVerifyBind(t *testing.T) {
	cases := []struct {
		name, fragment string
		edit           func(t *testing.T, p *mir.Program, f *midir.Func, in *midir.Inst)
	}{
		{"bind of another function", "bind yields a function of 0 parameters for add, which leaves 2", func(t *testing.T, p *mir.Program, f *midir.Func, in *midir.Inst) {
			in.Ent = entityNamed(t, p, sem.EntFn, "add")
		}},
		{"bind that is no function", "yields Int, not a function", func(t *testing.T, p *mir.Program, f *midir.Func, in *midir.Inst) {
			f.Locals[in.Dst].Type = sem.TyI64
		}},
		{"bind of another result", "which returns String", func(t *testing.T, p *mir.Program, f *midir.Func, in *midir.Inst) {
			f.Locals[in.Dst].Type = p.R.Types.Fn(nil, sem.TyI64, 0, false)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := mirfixture.Built(t, cellRulesSrc)
			f := mirfixture.FuncNamed(t, p, "main")
			c.edit(t, p, f, nth(t, f, midir.OpBind, 0))
			mirfixture.WantRule(t, p, "calls", c.fragment)
		})
	}
}

func TestVerifyBindingAfterLeaving(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"binding read after its drop", "main.pick", "variants", "which is not assigned on every path that reaches it", func(t *testing.T, p *mir.Program, f *midir.Func) {
			v := mirfixture.LocalNamed(t, f, "v")
			f.Blocks[1].Insts = append(f.Blocks[1].Insts, midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{v}})
		}},
		{"binding read after a move out", "main.pick", "variants", "copy reads %v, which is not assigned", func(t *testing.T, p *mir.Program, f *midir.Func) {
			v := mirfixture.LocalNamed(t, f, "v")
			f.Blocks[3].Insts = append([]midir.Inst{{Op: midir.OpMove, Dst: addLocal(f, "w", sem.TyI64), Args: []midir.LocalID{v}}}, f.Blocks[3].Insts...)
		}},
	})
}

func TestVerifyLentOnOnePath(t *testing.T) {
	runRuleCases(t, rulesSrc, true, []ruleCase{
		{"lent receiver returned through a join", "main.Point.moved", "droplent", "returns %r, a reference the caller still owns", func(t *testing.T, p *mir.Program, f *midir.Func) {
			r := addLocal(f, "r", f.Locals[f.Params[0]].Type)
			last := &f.Blocks[len(f.Blocks)-1]
			result := last.Term.Args[0]
			last.Insts = append(last.Insts, midir.Inst{Op: midir.OpAlias, Dst: r, Args: []midir.LocalID{f.Params[0]}}, midir.Inst{Op: midir.OpAlias, Dst: r, Args: []midir.LocalID{result}})
			last.Term.Args[0] = r
		}},
	})
}

func TestVerifyShapeAndTypeText(t *testing.T) {
	p := mirfixture.Program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyString}}
	body := func(in midir.Inst) *midir.Func {
		return mirfixture.Fn("text", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{in}, Term: mirfixture.Ret(0)})
	}
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnit, Dst: 0, Str: "x"}), "shape", `unit carries the text "x" it never reads`)
	expectRule(t, p, body(midir.Inst{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Str: "x"}), "shape", `const carries the text "x", which is not its number`)
	expectRule(t, p, body(midir.Inst{Op: midir.OpCopy, Dst: 0, Args: []midir.LocalID{0}, Lit: sem.Literal{Kind: sem.LitInt}}), "shape", "copy carries a literal it never reads")
	expectRule(t, p, body(midir.Inst{Op: midir.OpAlias, Dst: 0, Args: []midir.LocalID{1}}), "types", "alias from String into Int")
}

func TestVerifyLentWrapped(t *testing.T) {
	prog := mirbuild.Program(mirfixture.CheckSourceMir(t, rulesSrc))
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatal(err)
	}
	f := mirfixture.FuncNamed(t, prog, "main.Point.total")
	ref := prog.R.Types.Ref(f.Locals[f.Params[0]].Type, false)
	lent := addLocal(f, "b", ref)
	appendInst(f, midir.Inst{Op: midir.OpBorrow, Dst: lent, Args: []midir.LocalID{f.Params[0]}})
	appendInst(f, midir.Inst{Op: midir.OpNewCell, Dst: addLocal(f, "c", ref), Args: []midir.LocalID{lent}})
	mirfixture.WantRule(t, prog, "droplent", "in a cell that releases it")
}

func TestVerifyDirectReturns(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"return of a crossing local", "main.intoArray", "direct", "terminator reads the crossing local", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[f.Blocks[len(f.Blocks)-1].Term.Args[0]].Cross = true
		}},
		{"return in the form the signature lacks", "main.mkPoint", "direct", "in the form the signature does not carry", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.DSig.Ret = 0
		}},
	})
}

func TestVerifyStructureBeforeCalls(t *testing.T) {
	p := mirfixture.Built(t, rulesSrc)
	mirfixture.FuncNamed(t, p, "main.add").Params = []midir.LocalID{99}
	mirfixture.WantRule(t, p, "structure", "parameter 99 out of range")
}

func TestVerifyCrossingSource(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"source read after its crossing was consumed", "main.intoArray", "refs", "alias reads %t3 after its value was released", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpAlias, Dst: addLocal(f, "u", f.Locals[nth(t, f, midir.OpRecord, 0).Dst].Type), Args: []midir.LocalID{nth(t, f, midir.OpRecord, 0).Dst}})
		}},
	})
}

func TestVerifyOperandsAndTags(t *testing.T) {
	p := mirfixture.Program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyI64}, {Name: "t2", Type: sem.TyString}}
	body := func(in midir.Inst) *midir.Func {
		return mirfixture.Fn("ops", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{in}, Term: mirfixture.Ret(0)})
	}
	expectRule(t, p, body(midir.Inst{Op: midir.OpBinary, Dst: 0, Str: "+", Args: []midir.LocalID{1, 2}}), "types", "operator + of Int and String yields Int")
	expectRule(t, p, body(midir.Inst{Op: midir.OpBinary, Dst: 0, Str: "==", Args: []midir.LocalID{1, 2}}), "types", "comparison == yields Int, not Bool")
	never := []midir.Local{{Name: "t0", Type: sem.TyNever}}
	expectRule(t, p, mirfixture.Fn("never", sem.TyNever, never, midir.Block{Insts: []midir.Inst{{Op: midir.OpConst, Dst: 0, Type: sem.TyNever}}, Term: mirfixture.Ret(0)}), "types", "yields a value into a local of type Never")
	expectRule(t, p, body(midir.Inst{Op: midir.OpShell, Dst: 0, Strs: []string{"x"}}), "types", "shell plan yields Int, not a Plan")
	sqrt := primitiveOf(t, p, sem.TyF64, sem.TyF64)
	f64 := []midir.Local{{Name: "t0", Type: sem.TyF64}, {Name: "t1", Type: sem.TyF64}}
	call := midir.Inst{Op: midir.OpCall, Ent: sqrt, Dst: 0, Args: []midir.LocalID{1}}
	call.Prim = 7
	expectRule(t, p, mirfixture.Fn("tags", sem.TyF64, f64, midir.Block{Insts: []midir.Inst{call}, Term: mirfixture.Ret(0)}), "stdop", "runtime op tags")
	unit := midir.Inst{Op: midir.OpUnit, Dst: 0, DPrim: opdef.DPPow}
	expectRule(t, p, mirfixture.Fn("dprim", sem.TyUnit, []midir.Local{{Name: "t0", Type: sem.TyUnit}}, midir.Block{Insts: []midir.Inst{unit}, Term: mirfixture.Ret(0)}), "stdop", "direct result tag")
}

func TestVerifyAwaitConsumes(t *testing.T) {
	p := mirfixture.Planned(t, asyncRulesSrc)
	f := mirfixture.FuncNamed(t, p, "main.total")
	fut := nth(t, f, midir.OpAwait, 0).Args[0]
	appendInst(f, midir.Inst{Op: midir.OpAlias, Dst: addLocal(f, "u", f.Locals[fut].Type), Args: []midir.LocalID{fut}})
	mirfixture.WantRule(t, p, "refs", "reads %t4 after its value was released")
}

func TestVerifyGenericCallerArguments(t *testing.T) {
	runRuleCases(t, rulesSrc+"\nfn forward[T: Show](x: T) -> String { both(x) }\n", false, []ruleCase{
		{"type parameter where a function goes", "main.forward", "calls", "as argument 1 of main.both, which takes fn() -> String", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCall, 0)
			in.Args[0], in.Args[1] = in.Args[1], in.Args[0]
		}},
	})
}

func TestVerifyAsyncMark(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"closure marked async", "main.counter", "entities", "is marked async true", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpClosure, 0).Async = true
		}},
	})
	runRuleCases(t, asyncRulesSrc, false, []ruleCase{
		{"async function item not marked", "main", "entities", "is marked async false", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpFnItem, 0).Async = false
		}},
	})
}

func TestVerifyBorrowOutlivesOwner(t *testing.T) {
	runRuleCases(t, rulesSrc, true, []ruleCase{
		{"borrow read after its local was released", "main.keep", "refs", "reads %t4 after its value was released", func(t *testing.T, p *mir.Program, f *midir.Func) {
			b := nth(t, f, midir.OpBorrow, 0)
			rel := midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{b.Args[0]}}
			for bi := range f.Blocks {
				for k, in := range f.Blocks[bi].Insts {
					if in.Op == midir.OpBorrow {
						f.Blocks[bi].Insts = append(f.Blocks[bi].Insts[:k+1], append([]midir.Inst{rel}, f.Blocks[bi].Insts[k+1:]...)...)
						return
					}
				}
			}
		}},
	})
}

func TestVerifyFunctionValueResult(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"function value result of another type", "main.run", "calls", "takes String from a function of type fn(Int) -> Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCallValue, 0).Dst = addLocal(f, "s", sem.TyString)
		}},
	})
}

func TestVerifyScalarFieldInRef(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"scalar field read into a reference", "main.shift", "aggregates", "reads a field of Int into &Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[nth(t, f, midir.OpField, 0).Dst].Type = p.R.Types.Ref(sem.TyI64, false)
		}},
	})
}
