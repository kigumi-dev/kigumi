package verify_test

import (
	"strings"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

func TestOpDescriptorChecksOperandsAndResult(t *testing.T) {
	p := mirfixture.Program(t)
	id, _ := opdef.StdOpIdOf("ARRAY_LEN")
	locals := []midir.Local{{Name: "s", Type: sem.TyString}, {Name: "f", Type: sem.TyF64}}
	f := mirfixture.Fn("desc", sem.TyUnit, locals)
	call := midir.Inst{Op: midir.OpCall, Dst: 1, Args: []midir.LocalID{0}, Prim: opdef.PrimitiveId(id) + 1}
	if msg := mirverify.OpDescMismatch(p, f, call); !strings.Contains(msg, "argument 0 is a string") {
		t.Errorf("a String receiver for ARRAY_LEN: %q", msg)
	}
	call.Args = nil
	if msg := mirverify.OpDescMismatch(p, f, call); !strings.Contains(msg, "takes 1 arguments, got 0") {
		t.Errorf("a missing receiver for ARRAY_LEN: %q", msg)
	}
	f.Locals[0].Type = sem.TyUsize
	f.Locals[1].Type = sem.TyBool
	sqrt, _ := opdef.StdOpIdOf("MATH_SQRT")
	call = midir.Inst{Op: midir.OpCall, Dst: 1, Args: []midir.LocalID{0}, Prim: opdef.PrimitiveId(sqrt) + 1}
	if msg := mirverify.OpDescMismatch(p, f, call); !strings.Contains(msg, "argument 0 is a int") {
		t.Errorf("an integer for MATH_SQRT: %q", msg)
	}
	f.Locals[0].Type = sem.TyF64
	if msg := mirverify.OpDescMismatch(p, f, call); !strings.Contains(msg, "result is a bool") {
		t.Errorf("a Bool result for MATH_SQRT: %q", msg)
	}
	f.Locals[1].Type = sem.TyF64
	if msg := mirverify.OpDescMismatch(p, f, call); msg != "" {
		t.Errorf("a well-typed MATH_SQRT call: %q", msg)
	}
}
