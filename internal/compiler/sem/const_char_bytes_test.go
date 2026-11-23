package sem_test

import (
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// Simple const evaluation compares Char and Bytes exactly as a runtime
// expression does: Char by scalar value, Bytes by contents. The literals
// cover `!=`, the ends of the Char range, and Char/Bytes consts used as
// operands rather than literals.
func TestConstCharAndBytesComparisons(t *testing.T) {
	t.Parallel()
	src := `const CHAR_EQ: Bool = 'a' == 'a'
const CHAR_NE: Bool = 'a' != 'b'
const CHAR_NE_SAME: Bool = 'a' != 'a'
const CHAR_LT: Bool = 'a' < 'z'
const CHAR_LE: Bool = 'z' <= 'z'
const CHAR_GT: Bool = 'z' > 'a'
const CHAR_GE: Bool = 'a' >= 'z'
const CHAR_MIN_EQ: Bool = '\u{0}' == '\u{0}'
const CHAR_MIN_LT_MAX: Bool = '\u{0}' < '\u{10FFFF}'
const CHAR_MAX_GT_MIN: Bool = '\u{10FFFF}' > '\u{0}'
const CHAR_MAX_LT_MIN: Bool = '\u{10FFFF}' < '\u{0}'
const CHAR_A: Char = 'a'
const CHAR_Z: Char = 'z'
const CHAR_CONST_LT: Bool = CHAR_A < CHAR_Z
const CHAR_CONST_EQ: Bool = CHAR_A == CHAR_A
const BYTES_EQ: Bool = b"abc" == b"abc"
const BYTES_NE: Bool = b"abc" != b"abd"
const BYTES_NE_SAME: Bool = b"abc" != b"abc"
const BYTES_EMPTY: Bool = b"" == b""
const BYTES_EMPTY_NE: Bool = b"" != b"a"
const BYTES_LEN: Bool = b"ab" == b"abc"
const BYTES_NUL: Bool = b"\x00" == b"\x00"
const BYTES_LEFT: Bytes = b"abc"
const BYTES_RIGHT: Bytes = b"abd"
const BYTES_CONST_EQ: Bool = BYTES_LEFT == BYTES_LEFT
const BYTES_CONST_NE: Bool = BYTES_LEFT != BYTES_RIGHT
print "${CHAR_EQ}"
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", res.Render())
	}
	want := map[string]bool{
		"CHAR_EQ":         true,
		"CHAR_NE":         true,
		"CHAR_NE_SAME":    false,
		"CHAR_LT":         true,
		"CHAR_LE":         true,
		"CHAR_GT":         true,
		"CHAR_GE":         false,
		"CHAR_MIN_EQ":     true,
		"CHAR_MIN_LT_MAX": true,
		"CHAR_MAX_GT_MIN": true,
		"CHAR_MAX_LT_MIN": false,
		"CHAR_CONST_LT":   true,
		"CHAR_CONST_EQ":   true,
		"BYTES_EQ":        true,
		"BYTES_NE":        true,
		"BYTES_NE_SAME":   false,
		"BYTES_EMPTY":     true,
		"BYTES_EMPTY_NE":  true,
		"BYTES_LEN":       false,
		"BYTES_NUL":       true,
		"BYTES_CONST_EQ":  true,
		"BYTES_CONST_NE":  true,
	}
	for name, value := range want {
		ent := res.PackageMember(res.PackageByPath("main"), name)
		if ent == 0 {
			t.Fatalf("const %q not found", name)
		}
		lit, ok := res.ConstValueOf(ent)
		if !ok || lit.Kind != sem.LitBool {
			t.Fatalf("%s: value = %+v, %v, want a Bool literal", name, lit, ok)
		}
		if lit.Bool != value {
			t.Errorf("%s = %v, want %v", name, lit.Bool, value)
		}
	}
}
