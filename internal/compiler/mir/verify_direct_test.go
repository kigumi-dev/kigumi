package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

func directRule(t *testing.T, p *mir.Program, fragment string) {
	t.Helper()
	ve, ok := p.Verify().(*mir.Error)
	if !ok || ve.Rule != "direct" || !strings.Contains(ve.Msg, fragment) {
		t.Fatalf("got %v, want a direct error containing %q", ve, fragment)
	}
}

func TestVerifyDirectRejectsAMissingPart(t *testing.T) {
	p := planned(t, directPointSrc)
	f := funcNamed(t, p, "main.local")
	for bi := range f.Blocks {
		for k, in := range f.Blocks[bi].Insts {
			if in.Op == midir.OpField && f.IsDirect(in.Args[0]) {
				f.Blocks[bi].Insts[k].Index = 9
				if ve, ok := p.Verify().(*mir.Error); !ok || ve.Rule != "aggregates" || !strings.Contains(ve.Msg, "names field 9") {
					t.Fatalf("got %v, want an aggregates error naming field 9", ve)
				}
				return
			}
		}
	}
	t.Fatalf("no field read of a direct record:\n%s", p.Dump())
}

func TestVerifyDirectRejectsCallingARecord(t *testing.T) {
	p := planned(t, closureEnvSrc)
	f := funcNamed(t, p, "main.local")
	call, ok := firstOp(f, midir.OpCallValue)
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
			directRule(t, p, "calls a value that is not a closure")
			return
		}
	}
	t.Fatalf("no direct record local to call:\n%s", p.Dump())
}

func TestVerifyDirectRejectsAPrimitiveOperandItDoesNotTake(t *testing.T) {
	p := planned(t, `
fn widen(n: i32, s: String) -> Int {
    let w: Int = n.toInt()
    w + (s.len().toInt() || 0)
}

fn main() -> Void! {}
`)
	f := funcNamed(t, p, "main.widen")
	for bi := range f.Blocks {
		for k, in := range f.Blocks[bi].Insts {
			if in.Op == midir.OpCall && in.DPrim == opdef.DPConvert {
				f.Blocks[bi].Insts[k].Args = []midir.LocalID{f.Params[1]}
				f.Blocks[bi].Insts[k].DPrim = opdef.DPNone
				ve, ok := p.Verify().(*mir.Error)
				if !ok || (ve.Rule != "types" && ve.Rule != "direct") || !strings.Contains(ve.Msg, "String") {
					t.Fatalf("got %v, want a types or direct error naming the String operand", ve)
				}
				return
			}
		}
	}
	t.Fatalf("no direct conversion call:\n%s", p.Dump())
}

const crossingSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn take(p: Point) -> Int { p.x }

fn mk(x: Int) -> Point { Point { x: x, y: x } }

fn use(x: Int) -> Int {
    let p = mk(x)
    take(p)
}

fn main() -> Void! {}
`

// llgen copies an aggregate at parameters, call arguments and results, so each must match the signature.
func TestVerifyDirectRejectsAParameterInTheWrongForm(t *testing.T) {
	p := planned(t, crossingSrc)
	f := funcNamed(t, p, "main.take")
	if f.DSig.Param(0) == 0 || !f.IsDirect(f.Params[0]) {
		t.Fatalf("take does not carry its record unboxed:\n%s", p.Dump())
	}
	f.Direct[f.Params[0]] = false
	directRule(t, p, "form")
}

func TestVerifyDirectRejectsASignatureOfAnotherType(t *testing.T) {
	p := planned(t, crossingSrc)
	f := funcNamed(t, p, "main.take")
	f.DSig.Params[0] = p.R.Types.Option(f.DSig.Params[0])
	directRule(t, p, "another type")
}

func TestVerifyDirectRejectsAResultInTheWrongForm(t *testing.T) {
	p := planned(t, crossingSrc)
	f := funcNamed(t, p, "main.mk")
	f.DSig.Ret = 0
	directRule(t, p, "returns")
}

func TestVerifyDirectRejectsAnArgumentInTheWrongForm(t *testing.T) {
	p := planned(t, crossingSrc)
	f := funcNamed(t, p, "main.use")
	call, ok := firstOp(f, midir.OpCall)
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
	directRule(t, p, "reads")
}
