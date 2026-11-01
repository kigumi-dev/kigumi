package verify_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const cCallSrc = `
export(C) {
    pub fn twice(x: i32) -> i32 {
        x * 2
    }
}

fn applyTwice(f: extern(C) fn(i32) -> i32, x: i32) -> i32 {
    unsafe { f(f(x)) }
}

let f: extern(C) fn(i32) -> i32 = twice
print "${unsafe { f(21) }} ${applyTwice(twice, 5)}"
`

func TestVerifyCCallContract(t *testing.T) {
	runRuleCases(t, cCallSrc, false, []ruleCase{
		{"too many values", "main.applyTwice", "calls", "passes 2 values to a C function of 1", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCallC, 0)
			in.Args = append(in.Args, in.Args[1])
		}},
		{"too few values", "main.applyTwice", "calls", "passes 0 values to a C function of 1", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpCallC, 0)
			in.Args = in.Args[:1]
		}},
		{"argument of another type", "main.applyTwice", "calls", "as argument 0 of", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCallC, 0).Args[1] = newString(f)
		}},
		{"result of another type", "main.applyTwice", "calls", "from a C function returning", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[nth(t, f, midir.OpCallC, 0).Dst].Type = sem.TyString
		}},
		{"callee of another type", "main.applyTwice", "calls", "as extern(C) fn", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCallC, 0).Args[0] = newString(f)
		}},
		{"pointer type that is no C function", "main.applyTwice", "calls", "which is no C function pointer", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCallC, 0).Type = sem.TyString
		}},
	})
}

const asmSrc = `
fn shifted(x: u64) -> Out[u64] {
    unsafe {
        asm {
            "shl {value}, {n}"
            value: inout(reg) x
            n: const 3
            options nomem, nostack, pure, preserves_flags
        }
    }
}

let Out { value: sh } = shifted(5)
`

func TestVerifyAsmContract(t *testing.T) {
	runRuleCases(t, asmSrc, false, []ruleCase{
		{"too many inputs", "main.shifted", "calls", "passes 2 values to a block of 1 inputs", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpAsm, 0)
			in.Args = append(in.Args, in.Args[0])
		}},
		{"missing input", "main.shifted", "calls", "passes 0 values to a block of 1 inputs", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpAsm, 0).Args = nil
		}},
		{"input of another type", "main.shifted", "calls", "as input 0", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpAsm, 0).Args[0] = newString(f)
		}},
		{"result of another type", "main.shifted", "calls", "for a block that returns", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpAsm, 0).Dst = addLocal(f, "x", sem.TyString)
		}},
	})
}

func TestVerifyBuiltinContracts(t *testing.T) {
	builtin := func(name string, dst sem.TypeID, args ...func(*midir.Func) midir.LocalID) func(*testing.T, *mir.Program, *midir.Func) {
		return func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := midir.Inst{Op: midir.OpBuiltin, Str: name, Dst: addLocal(f, "x", dst)}
			for _, a := range args {
				in.Args = append(in.Args, a(f))
			}
			appendInst(f, in)
		}
	}
	str := func(f *midir.Func) midir.LocalID { return newString(f) }
	num := func(f *midir.Func) midir.LocalID { return f.Params[0] }
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"print of a number", "main.add", "builtins", "print takes Int as argument 0, not a String", builtin("print", sem.TyUnit, num)},
		{"print yielding a number", "main.add", "builtins", "print yields Int, not Void", builtin("print", sem.TyI64, str)},
		{"panic of a number", "main.add", "builtins", "panic takes Int as argument 0, not a String", builtin("panic", sem.TyNever, num)},
		{"fill by a string", "main.add", "builtins", "fixed.fill indexes with String", builtin("fixed.fill", sem.TyI64, num, str)},
		{"length that is no integer", "main.add", "builtins", "iter.len yields String, not an integer", builtin("iter.len", sem.TyString, str)},
		{"equality that is no bool", "main.add", "builtins", "direct.eq yields Int, not Bool", builtin("direct.eq", sem.TyI64, num, num)},
		{"too many operands", "main.add", "builtins", "host takes 0 to 0 arguments, got 1", builtin("host", sem.TyI64, num)},
	})
}

func TestVerifyPrimitiveReceiver(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"receiver of another object type", "main.pick", "types", "receiver wants", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpCall, 0).Args[0] = newString(f)
		}},
		{"receiver that is a type parameter", "main.pick", "types", "receiver wants", func(t *testing.T, p *mir.Program, f *midir.Func) {
			param := addLocal(f, "tp", p.R.Types.Param(p.R.Fn(entityNamed(t, p, sem.EntFn, "both")).TypeParams[0]))
			nth(t, f, midir.OpCall, 0).Args[0] = param
		}},
	})
}

func TestReceiverOfAnInterfaceOrTypeParameterOwner(t *testing.T) {
	p := mirfixture.Built(t, rulesSrc)
	tt := p.R.Types
	show := entityNamed(t, p, sem.EntInterface, "Show")
	iface := tt.Iface(show, nil)
	param := tt.Param(p.R.Fn(entityNamed(t, p, sem.EntFn, "both")).TypeParams[0])
	for _, c := range []struct {
		name       string
		owner, got sem.TypeID
		ok         bool
	}{
		{"interface to the same interface", iface, iface, true},
		{"interface to a borrow of it", iface, tt.Ref(iface, false), true},
		{"interface to a scalar", iface, sem.TyI64, false},
		{"interface to a string", iface, sem.TyString, false},
		{"interface to another interface", iface, tt.Iface(tt.ErrorEnt(), nil), false},
		{"type parameter to anything", param, sem.TyI64, true},
		{"concrete owner to a type parameter", sem.TyString, param, false},
		{"concrete owner to an interface", sem.TyString, iface, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if msg := mirverify.ReceiverMismatch(p, c.owner, c.got); (msg == "") != c.ok {
				t.Fatalf("receiver %s for owner %s: %q", p.R.TypeString(c.got), p.R.TypeString(c.owner), msg)
			}
		})
	}
}

func TestVerifyFieldWriteOnADirectSumType(t *testing.T) {
	runRuleCases(t, rulesSrc, true, []ruleCase{
		{"write into a sum type", "main.area", "aggregates", "names a field of Shape, a sum type", func(t *testing.T, p *mir.Program, f *midir.Func) {
			if !f.IsDirect(f.Params[0]) {
				t.Fatal("the base program should hold Shape unboxed")
			}
			n := addLocal(f, "n", sem.TyF64)
			appendInst(f, midir.Inst{Op: midir.OpConst, Dst: n, Type: sem.TyF64, Lit: sem.Literal{Kind: sem.LitFloat}})
			appendInst(f, midir.Inst{Op: midir.OpSetField, Args: []midir.LocalID{f.Params[0], n}, Index: 0})
		}},
	})
}
