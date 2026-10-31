package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
)

func TestVoidResultMainIsCalledDirectly(t *testing.T) {
	ir := llgen.EmitWith(plannedProgram(t, "fn main() -> Void! {}\n"), llgen.EmitOptions{})
	main := bodyOf(t, ir, "define i32 @main")
	if strings.Contains(ir, "main.main$boxed") {
		t.Error("the entry still goes through the boxed entry of main")
	}
	if !strings.Contains(main, "call %\"kg.adt.Result[Void,std/prelude.Error]\" @\"main.main\"()") {
		t.Errorf("the entry does not call the direct main:\n%s", main)
	}
	if strings.Count(main, "@\"kg.box.Result[Void,std/prelude.Error]\"") != 1 {
		t.Error("the Result is boxed on more than the Err path")
	}
}
