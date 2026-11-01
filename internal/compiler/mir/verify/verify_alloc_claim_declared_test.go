package verify_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

const declaredCopySrc = `
type Nm = {
    pub name String
    pub n Int
}

noalloc fn declared(n: Nm) -> Int {
    n.n
}

fn main() -> Void! {}
`

func withCopy(t *testing.T, p *mir.Program, name string) {
	t.Helper()
	f := mirfixture.FuncNamed(t, p, name)
	f.Locals = append(f.Locals, midir.Local{Name: "dup", Type: f.Locals[f.Params[0]].Type})
	cp := midir.Inst{Op: midir.OpShare, Copy: true, Dst: midir.LocalID(len(f.Locals) - 1), Args: []midir.LocalID{f.Params[0]}, Node: f.Blocks[0].Insts[0].Node}
	f.Blocks[0].Insts = append([]midir.Inst{cp}, f.Blocks[0].Insts...)
	if len(f.Direct) > 0 {
		f.Direct = append(f.Direct, false)
	}
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
}

func TestVerifyRejectsAnUnchargedCopyInADeclaredNoallocBody(t *testing.T) {
	p := mirfixture.Planned(t, declaredCopySrc)
	withCopy(t, p, "main.declared")
	err := mirverify.Program(p)
	ve, ok := err.(*mirverify.Error)
	if !ok || ve.Rule != "alloc-claim" || !strings.Contains(ve.Msg, "copies Nm") {
		t.Fatalf("got %v, want an alloc-claim fault for a copy sem never charged", err)
	}
}

func TestVerifyAcceptsADeclaredNoallocBodyWithoutCopies(t *testing.T) {
	p := mirfixture.Planned(t, declaredCopySrc)
	if err := mirverify.Program(p); err != nil {
		t.Fatal(err)
	}
}
