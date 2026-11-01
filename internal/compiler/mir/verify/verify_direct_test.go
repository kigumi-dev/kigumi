package verify_test

import (
	"strings"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

func TestVerifyDirectRejectsAMissingPart(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.DirectPointSrc)
	f := mirfixture.FuncNamed(t, p, "main.local")
	for bi := range f.Blocks {
		for k, in := range f.Blocks[bi].Insts {
			if in.Op == midir.OpField && f.IsDirect(in.Args[0]) {
				f.Blocks[bi].Insts[k].Index = 9
				if ve, ok := mirverify.Program(p).(*mirverify.Error); !ok || ve.Rule != "aggregates" || !strings.Contains(ve.Msg, "names field 9") {
					t.Fatalf("got %v, want an aggregates error naming field 9", ve)
				}
				return
			}
		}
	}
	t.Fatalf("no field read of a direct record:\n%s", p.Dump())
}

func TestVerifyDirectRejectsCallingARecord(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.ClosureEnvSrc)
	f := mirfixture.FuncNamed(t, p, "main.local")
	call, ok := mirfixture.FirstOp(f, midir.OpCallValue)
	if !ok {
		t.Fatalf("no value call:\n%s", p.Dump())
	}
	for i := range f.Locals {
		if l := midir.LocalID(i); f.IsDirect(l) && p.R.DirectAgg(f.Locals[l].Type) && !p.R.DirectClosure(f.Locals[l].Type) {
			for bi := range f.Blocks {
				for k, in := range f.Blocks[bi].Insts {
					if in.Op == midir.OpCallValue && in.Dst == call.Dst {
						f.Blocks[bi].Insts[k].Args = append([]midir.LocalID{l}, in.Args[1:]...)
					}
				}
			}
			mirfixture.DirectRule(t, p, "calls a value that is not a closure")
			return
		}
	}
	t.Fatalf("no direct record local to call:\n%s", p.Dump())
}

func TestVerifyDirectRejectsAPrimitiveOperandItDoesNotTake(t *testing.T) {
	p := mirfixture.Planned(t, `
fn widen(n: i32, s: String) -> Int {
    let w: Int = n.toInt()
    w + (s.len().toInt() || 0)
}

fn main() -> Void! {}
`)
	f := mirfixture.FuncNamed(t, p, "main.widen")
	for bi := range f.Blocks {
		for k, in := range f.Blocks[bi].Insts {
			if in.Op == midir.OpCall && in.DPrim == opdef.DPConvert {
				f.Blocks[bi].Insts[k].Args = []midir.LocalID{f.Params[1]}
				f.Blocks[bi].Insts[k].DPrim = opdef.DPNone
				ve, ok := mirverify.Program(p).(*mirverify.Error)
				if !ok || (ve.Rule != "types" && ve.Rule != "direct") || !strings.Contains(ve.Msg, "String") {
					t.Fatalf("got %v, want a types or direct error naming the String operand", ve)
				}
				return
			}
		}
	}
	t.Fatalf("no direct conversion call:\n%s", p.Dump())
}

// llgen copies an aggregate at parameters, call arguments and results, so each must match the signature.
func TestVerifyDirectRejectsAParameterInTheWrongForm(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	f := mirfixture.FuncNamed(t, p, "main.take")
	if f.DSig.Param(0) == 0 || !f.IsDirect(f.Params[0]) {
		t.Fatalf("take does not carry its record unboxed:\n%s", p.Dump())
	}
	f.Direct[f.Params[0]] = false
	mirfixture.DirectRule(t, p, "form")
}

func TestVerifyDirectRejectsASignatureOfAnotherType(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	f := mirfixture.FuncNamed(t, p, "main.take")
	f.DSig.Params[0] = p.R.Types.Option(f.DSig.Params[0])
	mirfixture.DirectRule(t, p, "another type")
}

func TestVerifyDirectRejectsAResultInTheWrongForm(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	f := mirfixture.FuncNamed(t, p, "main.mk")
	f.DSig.Ret = 0
	mirfixture.DirectRule(t, p, "returns")
}

func TestVerifyDirectRejectsAnArgumentInTheWrongForm(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	f := mirfixture.FuncNamed(t, p, "main.use")
	call, ok := mirfixture.FirstOp(f, midir.OpCall)
	for bi := range f.Blocks {
		for _, in := range f.Blocks[bi].Insts {
			if in.Op == midir.OpCall && p.R.Entity(in.Ent).Name == "take" {
				call, ok = in, true
			}
		}
	}
	if !ok || !f.IsDirect(call.Args[0]) {
		t.Fatalf("use does not hand take its record unboxed:\n%s", p.Dump())
	}
	f.Direct[call.Args[0]] = false
	mirfixture.DirectRule(t, p, "reads")
}
