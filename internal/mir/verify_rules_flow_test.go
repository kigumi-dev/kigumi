package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func TestVerifyRefs(t *testing.T) {
	runRuleCases(t, rulesSrc, true, []ruleCase{
		{"read of a dropped local", "main.greet", "refs", "alias reads %n after its value was released or moved out", func(t *testing.T, p *mir.Program, f *mir.Func) {
			appendInst(f, mir.Inst{Op: mir.OpAlias, Dst: addLocal(f, "u", sem.TyString), Args: []mir.LocalID{f.Params[0]}})
		}},
		{"return of a released value", "main.greet", "refs", "terminator reads %t1 after its value was released", func(t *testing.T, p *mir.Program, f *mir.Func) {
			appendInst(f, mir.Inst{Op: mir.OpRelease, Args: []mir.LocalID{nth(t, f, mir.OpInterp, 0).Dst}})
		}},
		{"release twice", "main.greet", "refs", "release of %t1, whose reference is already released", func(t *testing.T, p *mir.Program, f *mir.Func) {
			l := nth(t, f, mir.OpInterp, 0).Dst
			appendInst(f, mir.Inst{Op: mir.OpRelease, Args: []mir.LocalID{l}})
			appendInst(f, mir.Inst{Op: mir.OpRelease, Args: []mir.LocalID{l}})
		}},
		{"hand over twice", "main.feed", "refs", "call reads %t3 after its value was released", func(t *testing.T, p *mir.Program, f *mir.Func) {
			call := *nth(t, f, mir.OpCall, 0)
			appendInst(f, call)
		}},
	})
}

func TestVerifyReturnOwned(t *testing.T) {
	runRuleCases(t, rulesSrc, true, []ruleCase{
		{"return of a lent receiver", "main.Point.moved", "droplent", "returns %self, a reference the caller still owns", func(t *testing.T, p *mir.Program, f *mir.Func) {
			for bi := range f.Blocks {
				if f.Blocks[bi].Term.Op == mir.TermReturn {
					f.Blocks[bi].Term.Args[0] = f.Params[0]
				}
			}
		}},
	})
}

func TestVerifyVariantsFlow(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"payload on the wrong edge", "main.area", "variants", "which may hold Dot with 0", func(t *testing.T, p *mir.Program, f *mir.Func) {
			for bi := range f.Blocks {
				if f.Blocks[bi].Term.Op == mir.TermBranch {
					tg := f.Blocks[bi].Term.Targets
					tg[0], tg[1] = tg[1], tg[0]
					return
				}
			}
			t.Fatal("no branch")
		}},
		{"binding read where it was never made", "main.pick", "variants", "is not assigned on every path that reaches it", func(t *testing.T, p *mir.Program, f *mir.Func) {
			n := 0
			for bi := range f.Blocks {
				if f.Blocks[bi].Term.Op == mir.TermBranch {
					if n++; n == 2 {
						tg := f.Blocks[bi].Term.Targets
						tg[0], tg[1] = tg[1], tg[0]
						return
					}
				}
			}
			t.Fatal("no second branch")
		}},
		{"iteration step with no length test", "main.sum", "variants", "never tested it against the length", func(t *testing.T, p *mir.Program, f *mir.Func) {
			for bi := range f.Blocks {
				if term := &f.Blocks[bi].Term; term.Op == mir.TermBranch {
					*term = mir.Term{Op: mir.TermJump, Targets: term.Targets[:1]}
					return
				}
			}
			t.Fatal("no branch")
		}},
		{"binding returned where it was never made", "main.pick", "variants", "terminator reads %v, which is not assigned", func(t *testing.T, p *mir.Program, f *mir.Func) {
			for bi := range f.Blocks {
				if f.Blocks[bi].Term.Op == mir.TermReturn {
					f.Blocks[bi].Term.Args[0] = localNamed(t, f, "v")
				}
			}
		}},
		{"cell read after it was emptied", "main.keep", "variants", "after celltake emptied it", func(t *testing.T, p *mir.Program, f *mir.Func) {
			take := nth(t, f, mir.OpCellTake, 0)
			appendInst(f, mir.Inst{Op: mir.OpCellGet, Dst: addLocal(f, "x", f.Locals[take.Dst].Type), Args: []mir.LocalID{take.Args[0]}})
		}},
	})
}

func TestVerifyDefsMore(t *testing.T) {
	p := program(t)
	locals := []mir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyI64}}
	f := fn("termread", sem.TyI64, locals, mir.Block{Insts: []mir.Inst{{Op: mir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}}, Term: ret(1)})
	expectRule(t, p, f, "defs", "terminator reads %t1 before any definition")
}
