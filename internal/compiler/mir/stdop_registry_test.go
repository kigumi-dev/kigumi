package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func TestOpDescriptorChecksOperandsAndResult(t *testing.T) {
	p := program(t)
	id, _ := opdef.StdOpIdOf("ARRAY_LEN")
	locals := []mir.Local{{Name: "s", Type: sem.TyString}, {Name: "f", Type: sem.TyF64}}
	f := fn("desc", sem.TyUnit, locals)
	call := mir.Inst{Op: mir.OpCall, Dst: 1, Args: []mir.LocalID{0}, Prim: opdef.PrimitiveId(id) + 1}
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "argument 0 is a string") {
		t.Errorf("a String receiver for ARRAY_LEN: %q", msg)
	}
	call.Args = nil
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "takes 1 arguments, got 0") {
		t.Errorf("a missing receiver for ARRAY_LEN: %q", msg)
	}
	f.Locals[0].Type = sem.TyUsize
	f.Locals[1].Type = sem.TyBool
	sqrt, _ := opdef.StdOpIdOf("MATH_SQRT")
	call = mir.Inst{Op: mir.OpCall, Dst: 1, Args: []mir.LocalID{0}, Prim: opdef.PrimitiveId(sqrt) + 1}
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "argument 0 is a int") {
		t.Errorf("an integer for MATH_SQRT: %q", msg)
	}
	f.Locals[0].Type = sem.TyF64
	if msg := p.OpDescMismatch(f, call); !strings.Contains(msg, "result is a bool") {
		t.Errorf("a Bool result for MATH_SQRT: %q", msg)
	}
	f.Locals[1].Type = sem.TyF64
	if msg := p.OpDescMismatch(f, call); msg != "" {
		t.Errorf("a well-typed MATH_SQRT call: %q", msg)
	}
}
