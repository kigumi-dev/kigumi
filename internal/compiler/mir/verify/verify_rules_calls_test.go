package verify_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const asyncRulesSrc = `
async fn twice(x: Int) -> Int { x * 2 }

async fn total(a: Int, b: Int) -> Int {
    let x = await twice(a)
    let y = await twice(b)
    x + y
}

let _ = total(1, 2)
`

func TestVerifyAsync(t *testing.T) {
	cases := []struct {
		name, fragment string
		edit           func(t *testing.T, p *mir.Program, f *midir.Func)
	}{
		{"await outside async", "await in a function that is not async", func(t *testing.T, p *mir.Program, f *midir.Func) { f.Async = false }},
		{"suspension out of range", "suspension point 9 outside 1..2", func(t *testing.T, p *mir.Program, f *midir.Func) { nth(t, f, midir.OpAwait, 0).Index = 9 }},
		{"suspension twice", "awaited twice", func(t *testing.T, p *mir.Program, f *midir.Func) { nth(t, f, midir.OpAwait, 1).Index = 1 }},
		{"suspension count", "5 suspension points but 2 awaits", func(t *testing.T, p *mir.Program, f *midir.Func) { f.Suspends = 5 }},
		{"frame slots", "frame slots for", func(t *testing.T, p *mir.Program, f *midir.Func) { f.Slot = f.Slot[:1] }},
		{"slot out of the frame", "in slot", func(t *testing.T, p *mir.Program, f *midir.Func) { f.Slot[0] = f.Frame + 3 }},
		{"slot shared", "holds two locals", func(t *testing.T, p *mir.Program, f *midir.Func) {
			first := -1
			for i, s := range f.Slot {
				if s < 0 {
					continue
				}
				if first < 0 {
					first = i
				} else {
					f.Slot[i] = f.Slot[first]
					return
				}
			}
			t.Fatal("fewer than two frame locals")
		}},
		{"cleanup lists", "cleanup lists for", func(t *testing.T, p *mir.Program, f *midir.Func) { f.Cleanup = f.Cleanup[:1] }},
		{"duplicate panic cleanup", "drops local", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.PanicDrop = []midir.LocalID{0, 0}
		}},
		{"cleanup of a local with no slot", "has no frame slot", func(t *testing.T, p *mir.Program, f *midir.Func) {
			for i, s := range f.Slot {
				if s < 0 {
					f.Cleanup[1] = append(f.Cleanup[1], midir.LocalID(i))
					return
				}
			}
			t.Fatal("every local has a slot")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := mirfixture.Planned(t, asyncRulesSrc)
			c.edit(t, p, mirfixture.FuncNamed(t, p, "main.total"))
			mirfixture.WantRule(t, p, "async", c.fragment)
		})
	}
	t.Run("completion outside async", func(t *testing.T) {
		p := mirfixture.Built(t, rulesSrc)
		f := mirfixture.FuncNamed(t, p, "main.add")
		appendInst(f, midir.Inst{Op: midir.OpFutureDone, Args: []midir.LocalID{0}})
		mirfixture.WantRule(t, p, "async", "completion of a function that is not async")
	})
}

func TestVerifyParams(t *testing.T) {
	p := mirfixture.Built(t, rulesSrc)
	f := mirfixture.FuncNamed(t, p, "main.add")
	f.Params[0], f.Params[1] = f.Params[1], f.Params[0]
	mirfixture.WantRule(t, p, "calls", "parameter 0 is %b, not the a")
	f.Params[0], f.Params[1] = f.Params[1], f.Params[0]
	f.Params = f.Params[:1]
	mirfixture.WantRule(t, p, "calls", "takes 1 parameters, but add declares 2")
}

func TestVerifyEntities(t *testing.T) {
	cases := []struct {
		name, fragment string
		edit           func(t *testing.T, p *mir.Program, f *midir.Func)
	}{
		{"call of a type", "which is not a function", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCall, 0).Ent = entityNamed(t, p, sem.EntType, "Point")
		}},
		{"variant test of a type", "which is not a variant", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpIsVariant, 0).Ent = entityNamed(t, p, sem.EntType, "Point")
		}},
		{"record of too few fields", "record of 2 fields built from 1 values", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpRecord, 0)
			in.Args = in.Args[:1]
		}},
		{"closure of a function", "a lowered closure, or a method for a method value", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpClosure, 0).Ent = entityNamed(t, p, sem.EntFn, "add")
		}},
		{"static of a function", "which is not a static", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpGlobalGet, Dst: addLocal(f, "g", sem.TyI64), Ent: entityNamed(t, p, sem.EntFn, "add")})
		}},
		{"negative field", "field index -1", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpField, 0).Index = -1
		}},
		{"asm without a table", "asm without its operand table", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpAsm, Dst: addLocal(f, "a", sem.TyI64), Type: sem.TyI64})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := mirfixture.Built(t, rulesSrc)
			name := map[string]string{"call of a type": "main", "variant test of a type": "main.area", "record of too few fields": "main.shift", "closure of a function": "main.counter", "negative field": "main.shift"}[c.name]
			if name == "" {
				name = "main.add"
			}
			c.edit(t, p, mirfixture.FuncNamed(t, p, name))
			mirfixture.WantRule(t, p, "entities", c.fragment)
		})
	}
}

func TestVerifyCalls(t *testing.T) {
	str := func(f *midir.Func) midir.LocalID {
		s := addLocal(f, "s", sem.TyString)
		f.Blocks[0].Insts = append([]midir.Inst{{Op: midir.OpConst, Dst: s, Type: sem.TyString, Lit: sem.Literal{Kind: sem.LitString}}}, f.Blocks[0].Insts...)
		return s
	}
	cases := []struct {
		name, fn, fragment string
		edit               func(t *testing.T, p *mir.Program, f *midir.Func)
	}{
		{"too few arguments", "main", "passes 1 values to main.add, which takes 2", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCall, 0)
			in.Args = in.Args[:1]
		}},
		{"argument of another type", "main", "passes String as argument 0 of main.add", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCall, 0).Args[0] = str(f)
		}},
		{"result of another type", "main", "takes String from main.add, which returns Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[nth(t, f, midir.OpCall, 0).Dst].Type = sem.TyString
		}},
		{"function value of too few arguments", "main.run", "passes 0 values to a function of 1", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCallValue, 0)
			in.Args = in.Args[:1]
		}},
		{"function value argument of another type", "main.run", "passes String as argument 0 of", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCallValue, 0).Args[1] = str(f)
		}},
		{"witness of too few arguments", "main.both", "values to the witness of show", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCallValue, 0)
			in.Args = in.Args[:1]
		}},
		{"witness handed the witness", "main.both", "hands the witness for T a", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCallValue, 0)
			in.Args[1] = in.Args[0]
		}},
		{"closure of too many captures", "main.counter", "captures 2 values for a body of 1 parameters", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpClosure, 0)
			in.Args = append(in.Args, in.Args[0])
		}},
		{"closure capture of another type", "main.counter", "captures String for a parameter of Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpClosure, 0).Args[0] = str(f)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := mirfixture.Built(t, rulesSrc)
			c.edit(t, p, mirfixture.FuncNamed(t, p, c.fn))
			mirfixture.WantRule(t, p, "calls", c.fragment)
		})
	}
	t.Run("primitive of too few arguments", func(t *testing.T) {
		p := mirfixture.Program(t)
		sqrt := primitiveOf(t, p, sem.TyF64, sem.TyF64)
		f := mirfixture.Fn("prim", sem.TyF64, []midir.Local{{Name: "t0", Type: sem.TyF64}}, midir.Block{Insts: []midir.Inst{{Op: midir.OpCall, Ent: sqrt, Dst: 0}}, Term: mirfixture.Ret(0)})
		expectRule(t, p, f, "calls", "passes 0 values to sqrt, which takes 1")
	})
	t.Run("future of the wrong arity", func(t *testing.T) {
		p := mirfixture.Planned(t, asyncRulesSrc)
		f := mirfixture.FuncNamed(t, p, "main")
		in := nth(t, f, midir.OpFutureNew, 0)
		in.Args = in.Args[:1]
		mirfixture.WantRule(t, p, "calls", "values to main.total, which takes 2")
	})
}
