package vm_test

import (
	"errors"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/sem"
)

// A comptime result that is not a finite float has no constant literal to
// fold into, so the fold boundary must report a typed error instead of
// letting big.NewFloat panic. Scalar, record-field, fmod and sqrt results
// all pass through that one boundary; Inf follows the same rule as NaN.
func TestComptimeNonFiniteResultIsAnError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		src   string
		typ   string
		field string
	}{
		{
			name: "f64 scalar NaN",
			src:  "pure fn divide(a: Float, b: Float) -> Float { a / b }\nconst VALUE: Float = comptime divide(0.0, 0.0)\nprint \"${VALUE}\"\n",
			typ:  "Float",
		},
		{
			name: "f32 scalar NaN",
			src:  "pure fn divide(a: f32, b: f32) -> f32 { a / b }\nconst VALUE: f32 = comptime divide(0.0, 0.0)\nprint \"${VALUE}\"\n",
			typ:  "f32",
		},
		{
			name:  "record field NaN",
			src:   "type Value = { number Float }\npure fn make(a: Float, b: Float) -> Value { Value { number: a / b } }\nconst VALUE: Value = comptime make(0.0, 0.0)\nprint \"${VALUE.number}\"\n",
			typ:   "Float",
			field: "number",
		},
		{
			name: "fmod by zero NaN",
			src:  "pure fn remainder(a: Float, b: Float) -> Float { a % b }\nconst VALUE: Float = comptime remainder(1.0, 0.0)\nprint \"${VALUE}\"\n",
			typ:  "Float",
		},
		{
			name: "sqrt of a negative NaN",
			src:  "import math from std/math\nconst VALUE: Float = comptime math.sqrt(-1.0)\nprint \"${VALUE}\"\n",
			typ:  "Float",
		},
		{
			name: "f64 scalar +Inf",
			src:  "pure fn divide(a: Float, b: Float) -> Float { a / b }\nconst VALUE: Float = comptime divide(1.0, 0.0)\nprint \"${VALUE}\"\n",
			typ:  "Float",
		},
		{
			name: "f64 scalar -Inf",
			src:  "pure fn divide(a: Float, b: Float) -> Float { a / b }\nconst VALUE: Float = comptime divide(-1.0, 0.0)\nprint \"${VALUE}\"\n",
			typ:  "Float",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("EvalComptime panicked: %v", r)
				}
			}()
			_, err := evalOneComptime(t, c.src)
			if err == nil {
				t.Fatal("want a non-finite diagnostic, got success")
			}
			var nonFinite *vm.NonFiniteError
			if !errors.As(err, &nonFinite) {
				t.Fatalf("error = %v (%T), want *vm.NonFiniteError", err, err)
			}
			if nonFinite.Type != c.typ {
				t.Errorf("type = %q, want %q", nonFinite.Type, c.typ)
			}
			if nonFinite.FieldPath != c.field {
				t.Errorf("field path = %q, want %q", nonFinite.FieldPath, c.field)
			}
		})
	}
}

// A NaN used only inside the block is fine: it may produce a Bool or a
// finite float, and only the folded result must be a finite literal.
func TestComptimeNaNInsideBlockFoldsItsResult(t *testing.T) {
	t.Parallel()
	t.Run("Bool", func(t *testing.T) {
		lit, err := evalOneComptime(t, "pure fn compare(a: Float, b: Float) -> Bool { let nan = a / b; nan == nan }\nconst VALUE: Bool = comptime compare(0.0, 0.0)\nprint \"${VALUE}\"\n")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if lit.Kind != sem.LitBool || lit.Bool {
			t.Fatalf("literal = %+v, want Bool false", lit)
		}
	})
	t.Run("finite Float", func(t *testing.T) {
		lit, err := evalOneComptime(t, "pure fn pick(a: Float, b: Float) -> Float {\n    let nan = a / b\n    if nan == nan { 1.0 } else { 2.0 }\n}\nlet x = comptime pick(0.0, 0.0)\nprint \"${x}\"\n")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if lit.Kind != sem.LitFloat || lit.Float == nil {
			t.Fatalf("literal = %+v, want a Float literal", lit)
		}
		if f, _ := lit.Float.Float64(); f != 2 {
			t.Fatalf("literal = %v, want 2", f)
		}
	})
}
