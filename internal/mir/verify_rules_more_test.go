package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
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
		{"take of a received cell", "main.Point.bump", "cellops", "celltake empties %self, a cell this frame did not make", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpCellGet, 0).Op = mir.OpCellTake
		}},
		{"wrap of a received cell", "main.Point.bump", "cellops", "newcell wraps %self, which is already a cell", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpCellGet, 0).Op = mir.OpNewCell
		}},
		{"borrow handed to a mut receiver", "main.drive", "cellops", "call hands %t4, which is no cell, to a mut receiver", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpCall, 0).Args[0] = nth(t, f, mir.OpBorrow, 0).Dst
		}},
		{"cell stored in itself", "main.Point.bump", "cellops", "cellset reads the cell %self as a plain value", func(t *testing.T, p *mir.Program, f *mir.Func) {
			appendInst(f, mir.Inst{Op: mir.OpCellSet, Args: []mir.LocalID{f.Params[0], f.Params[0]}})
		}},
		{"cell read as a value", "main.Point.bump", "cellops", "field reads the cell %self as a plain value", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpField, 0).Args[0] = f.Params[0]
		}},
	})
}

func TestVerifyFutureRules(t *testing.T) {
	future := func(p *mir.Program, ret sem.TypeID) sem.TypeID {
		return p.R.Types.Named(p.R.Types.FutureEnt(), []sem.TypeID{ret})
	}
	cases := []struct {
		name, fragment string
		edit           func(p *mir.Program, f *mir.Func, in *mir.Inst)
	}{
		{"future of a value", "which is not an async function item", func(p *mir.Program, f *mir.Func, in *mir.Inst) { in.Args[0] = in.Args[1] }},
		{"future of another kind", "yields Int, not a Future", func(p *mir.Program, f *mir.Func, in *mir.Inst) { f.Locals[in.Dst].Type = sem.TyI64 }},
		{"future of another result", "which returns Int", func(p *mir.Program, f *mir.Func, in *mir.Inst) {
			f.Locals[in.Dst].Type = future(p, sem.TyString)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := planned(t, asyncRulesSrc)
			f := funcNamed(t, p, "main")
			c.edit(p, f, nth(t, f, mir.OpFutureNew, 0))
			wantRule(t, p, "calls", c.fragment)
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
		{"extra argument", "main.absolute", "calls", "passes 2 values to labs, which takes 1", func(t *testing.T, p *mir.Program, f *mir.Func) {
			in := nth(t, f, mir.OpCall, 0)
			in.Args = append(in.Args, in.Args[0])
		}},
		{"argument of another type", "main.absolute", "calls", "passes String as argument 0 of labs, which takes Int", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpCall, 0).Args[0] = newString(f)
		}},
		{"result of another type", "main.absolute", "calls", "takes String from labs, which returns Int", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpCall, 0).Dst = addLocal(f, "s", sem.TyString)
		}},
	})
}

func TestVerifyBind(t *testing.T) {
	cases := []struct {
		name, fragment string
		edit           func(t *testing.T, p *mir.Program, f *mir.Func, in *mir.Inst)
	}{
		{"bind of another function", "bind yields a function of 0 parameters for add, which leaves 2", func(t *testing.T, p *mir.Program, f *mir.Func, in *mir.Inst) {
			in.Ent = entityNamed(t, p, sem.EntFn, "add")
		}},
		{"bind that is no function", "yields Int, not a function", func(t *testing.T, p *mir.Program, f *mir.Func, in *mir.Inst) {
			f.Locals[in.Dst].Type = sem.TyI64
		}},
		{"bind of another result", "which returns String", func(t *testing.T, p *mir.Program, f *mir.Func, in *mir.Inst) {
			f.Locals[in.Dst].Type = p.R.Types.Fn(nil, sem.TyI64, 0, false)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := built(t, cellRulesSrc)
			f := funcNamed(t, p, "main")
			c.edit(t, p, f, nth(t, f, mir.OpBind, 0))
			wantRule(t, p, "calls", c.fragment)
		})
	}
}

func TestVerifyBindingAfterLeaving(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"binding read after its drop", "main.pick", "variants", "which is not assigned on every path that reaches it", func(t *testing.T, p *mir.Program, f *mir.Func) {
			v := localNamed(t, f, "v")
			f.Blocks[1].Insts = append(f.Blocks[1].Insts, mir.Inst{Op: mir.OpDrop, Args: []mir.LocalID{v}})
		}},
		{"binding read after a move out", "main.pick", "variants", "copy reads %v, which is not assigned", func(t *testing.T, p *mir.Program, f *mir.Func) {
			v := localNamed(t, f, "v")
			f.Blocks[3].Insts = append([]mir.Inst{{Op: mir.OpMove, Dst: addLocal(f, "w", sem.TyI64), Args: []mir.LocalID{v}}}, f.Blocks[3].Insts...)
		}},
	})
}

func TestVerifyLentOnOnePath(t *testing.T) {
	runRuleCases(t, rulesSrc, true, []ruleCase{
		{"lent receiver returned through a join", "main.Point.moved", "droplent", "returns %r, a reference the caller still owns", func(t *testing.T, p *mir.Program, f *mir.Func) {
			r := addLocal(f, "r", f.Locals[f.Params[0]].Type)
			last := &f.Blocks[len(f.Blocks)-1]
			result := last.Term.Args[0]
			last.Insts = append(last.Insts, mir.Inst{Op: mir.OpAlias, Dst: r, Args: []mir.LocalID{f.Params[0]}}, mir.Inst{Op: mir.OpAlias, Dst: r, Args: []mir.LocalID{result}})
			last.Term.Args[0] = r
		}},
	})
}

func TestVerifyShapeAndTypeText(t *testing.T) {
	p := program(t)
	locals := []mir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyString}}
	body := func(in mir.Inst) *mir.Func {
		return fn("text", sem.TyI64, locals, mir.Block{Insts: []mir.Inst{in}, Term: ret(0)})
	}
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnit, Dst: 0, Str: "x"}), "shape", `unit carries the text "x" it never reads`)
	expectRule(t, p, body(mir.Inst{Op: mir.OpConst, Dst: 0, Type: sem.TyI64, Str: "x"}), "shape", `const carries the text "x", which is not its number`)
	expectRule(t, p, body(mir.Inst{Op: mir.OpAlias, Dst: 0, Args: []mir.LocalID{1}}), "types", "alias from String into Int")
}

func TestVerifyLentWrapped(t *testing.T) {
	prog := mir.Build(checkSourceMir(t, rulesSrc))
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatal(err)
	}
	f := funcNamed(t, prog, "main.Point.total")
	ref := prog.R.Types.Ref(f.Locals[f.Params[0]].Type, false)
	lent := addLocal(f, "b", ref)
	appendInst(f, mir.Inst{Op: mir.OpBorrow, Dst: lent, Args: []mir.LocalID{f.Params[0]}})
	appendInst(f, mir.Inst{Op: mir.OpNewCell, Dst: addLocal(f, "c", ref), Args: []mir.LocalID{lent}})
	wantRule(t, prog, "droplent", "in a cell that releases it")
}

func TestVerifyDirectReturns(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"return of a crossing local", "main.intoArray", "direct", "terminator reads the crossing local", func(t *testing.T, p *mir.Program, f *mir.Func) {
			f.Locals[f.Blocks[len(f.Blocks)-1].Term.Args[0]].Cross = true
		}},
		{"return in the form the signature lacks", "main.mkPoint", "direct", "in the form the signature does not carry", func(t *testing.T, p *mir.Program, f *mir.Func) {
			f.DSig.Ret = 0
		}},
	})
}

func TestVerifyStructureBeforeCalls(t *testing.T) {
	p := built(t, rulesSrc)
	funcNamed(t, p, "main.add").Params = []mir.LocalID{99}
	wantRule(t, p, "structure", "parameter 99 out of range")
}

func TestVerifyCrossingSource(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"source read after its crossing was consumed", "main.intoArray", "refs", "alias reads %t3 after its value was released", func(t *testing.T, p *mir.Program, f *mir.Func) {
			appendInst(f, mir.Inst{Op: mir.OpAlias, Dst: addLocal(f, "u", f.Locals[nth(t, f, mir.OpRecord, 0).Dst].Type), Args: []mir.LocalID{nth(t, f, mir.OpRecord, 0).Dst}})
		}},
	})
}

func TestVerifyOperandsAndTags(t *testing.T) {
	p := program(t)
	locals := []mir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyI64}, {Name: "t2", Type: sem.TyString}}
	body := func(in mir.Inst) *mir.Func {
		return fn("ops", sem.TyI64, locals, mir.Block{Insts: []mir.Inst{in}, Term: ret(0)})
	}
	expectRule(t, p, body(mir.Inst{Op: mir.OpBinary, Dst: 0, Str: "+", Args: []mir.LocalID{1, 2}}), "types", "operator + of Int and String yields Int")
	expectRule(t, p, body(mir.Inst{Op: mir.OpBinary, Dst: 0, Str: "==", Args: []mir.LocalID{1, 2}}), "types", "comparison == yields Int, not Bool")
	never := []mir.Local{{Name: "t0", Type: sem.TyNever}}
	expectRule(t, p, fn("never", sem.TyNever, never, mir.Block{Insts: []mir.Inst{{Op: mir.OpConst, Dst: 0, Type: sem.TyNever}}, Term: ret(0)}), "types", "yields a value into a local of type Never")
	expectRule(t, p, body(mir.Inst{Op: mir.OpShell, Dst: 0, Strs: []string{"x"}}), "types", "shell plan yields Int, not a Plan")
	sqrt := primitiveOf(t, p, sem.TyF64, sem.TyF64)
	f64 := []mir.Local{{Name: "t0", Type: sem.TyF64}, {Name: "t1", Type: sem.TyF64}}
	call := mir.Inst{Op: mir.OpCall, Ent: sqrt, Dst: 0, Args: []mir.LocalID{1}}
	call.Prim = 7
	expectRule(t, p, fn("tags", sem.TyF64, f64, mir.Block{Insts: []mir.Inst{call}, Term: ret(0)}), "stdop", "runtime op tags")
	unit := mir.Inst{Op: mir.OpUnit, Dst: 0, DPrim: mir.DPPow}
	expectRule(t, p, fn("dprim", sem.TyUnit, []mir.Local{{Name: "t0", Type: sem.TyUnit}}, mir.Block{Insts: []mir.Inst{unit}, Term: ret(0)}), "stdop", "direct result tag")
}

func TestVerifyAwaitConsumes(t *testing.T) {
	p := planned(t, asyncRulesSrc)
	f := funcNamed(t, p, "main.total")
	fut := nth(t, f, mir.OpAwait, 0).Args[0]
	appendInst(f, mir.Inst{Op: mir.OpAlias, Dst: addLocal(f, "u", f.Locals[fut].Type), Args: []mir.LocalID{fut}})
	wantRule(t, p, "refs", "reads %t4 after its value was released")
}

func TestVerifyGenericCallerArguments(t *testing.T) {
	runRuleCases(t, rulesSrc+"\nfn forward[T: Show](x: T) -> String { both(x) }\n", false, []ruleCase{
		{"type parameter where a function goes", "main.forward", "calls", "as argument 1 of main.both, which takes fn() -> String", func(t *testing.T, p *mir.Program, f *mir.Func) {
			in := nth(t, f, mir.OpCall, 0)
			in.Args[0], in.Args[1] = in.Args[1], in.Args[0]
		}},
	})
}

func TestVerifyAsyncMark(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"closure marked async", "main.counter", "entities", "is marked async true", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpClosure, 0).Async = true
		}},
	})
	runRuleCases(t, asyncRulesSrc, false, []ruleCase{
		{"async function item not marked", "main", "entities", "is marked async false", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpFnItem, 0).Async = false
		}},
	})
}

func TestVerifyBorrowOutlivesOwner(t *testing.T) {
	runRuleCases(t, rulesSrc, true, []ruleCase{
		{"borrow read after its local was released", "main.keep", "refs", "reads %t4 after its value was released", func(t *testing.T, p *mir.Program, f *mir.Func) {
			b := nth(t, f, mir.OpBorrow, 0)
			rel := mir.Inst{Op: mir.OpRelease, Args: []mir.LocalID{b.Args[0]}}
			for bi := range f.Blocks {
				for k, in := range f.Blocks[bi].Insts {
					if in.Op == mir.OpBorrow {
						f.Blocks[bi].Insts = append(f.Blocks[bi].Insts[:k+1], append([]mir.Inst{rel}, f.Blocks[bi].Insts[k+1:]...)...)
						return
					}
				}
			}
		}},
	})
}

func TestVerifyFunctionValueResult(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"function value result of another type", "main.run", "calls", "takes String from a function of type fn(Int) -> Int", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpCallValue, 0).Dst = addLocal(f, "s", sem.TyString)
		}},
	})
}

func TestVerifyScalarFieldInRef(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"scalar field read into a reference", "main.shift", "aggregates", "reads a field of Int into &Int", func(t *testing.T, p *mir.Program, f *mir.Func) {
			f.Locals[nth(t, f, mir.OpField, 0).Dst].Type = p.R.Types.Ref(sem.TyI64, false)
		}},
	})
}
