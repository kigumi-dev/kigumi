package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
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
	f := funcNamed(t, p, name)
	f.Locals = append(f.Locals, mir.Local{Name: "dup", Type: f.Locals[f.Params[0]].Type})
	cp := mir.Inst{Op: mir.OpShare, Copy: true, Dst: mir.LocalID(len(f.Locals) - 1), Args: []mir.LocalID{f.Params[0]}, Node: f.Blocks[0].Insts[0].Node}
	f.Blocks[0].Insts = append([]mir.Inst{cp}, f.Blocks[0].Insts...)
	if len(f.Direct) > 0 {
		f.Direct = append(f.Direct, false)
	}
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
}

func TestVerifyRejectsAnUnchargedCopyInADeclaredNoallocBody(t *testing.T) {
	p := planned(t, declaredCopySrc)
	withCopy(t, p, "main.declared")
	err := p.Verify()
	ve, ok := err.(*mir.Error)
	if !ok || ve.Rule != "alloc-claim" || !strings.Contains(ve.Msg, "copies Nm") {
		t.Fatalf("got %v, want an alloc-claim fault for a copy sem never charged", err)
	}
}

func TestVerifyAcceptsADeclaredNoallocBodyWithoutCopies(t *testing.T) {
	p := planned(t, declaredCopySrc)
	if err := p.Verify(); err != nil {
		t.Fatal(err)
	}
}
