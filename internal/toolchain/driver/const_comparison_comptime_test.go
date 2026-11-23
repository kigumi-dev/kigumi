package driver_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A non-finite comptime result must be an ordinary positioned diagnostic at
// the block, not a panic or an internal error. Scalar and record results go
// through the same fold boundary, and Inf follows the same rule as NaN.
func TestComptimeNonFiniteIsAPositionedDiagnostic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "scalar NaN",
			src:  "pure fn divide(a: Float, b: Float) -> Float { a / b }\nconst VALUE: Float = comptime divide(0.0, 0.0)\nprint \"${VALUE}\"\n",
			want: []string{"error[E2322]", "main.kg:2:", "produced NaN", "not a finite `Float` constant"},
		},
		{
			name: "record field NaN",
			src:  "type Value = { number Float }\npure fn make(a: Float, b: Float) -> Value { Value { number: a / b } }\nconst VALUE: Value = comptime make(0.0, 0.0)\nprint \"${VALUE.number}\"\n",
			want: []string{"error[E2322]", "main.kg:3:", "produced NaN in field `number`", "not a finite `Float` constant"},
		},
		{
			name: "scalar +Inf",
			src:  "pure fn divide(a: Float, b: Float) -> Float { a / b }\nconst VALUE: Float = comptime divide(1.0, 0.0)\nprint \"${VALUE}\"\n",
			want: []string{"error[E2322]", "main.kg:2:", "produced +Inf", "not a finite `Float` constant"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := moduleOf(t, "nonfinite", map[string]string{"main.kg": c.src}, driver.LoadOptions{})
			a, err := driver.Analyze(m)
			if err != nil {
				t.Fatal(err)
			}
			if !a.Res.HasErrors() {
				t.Fatal("want a compile-time error")
			}
			out := a.Res.Render()
			for _, want := range c.want {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in:\n%s", want, out)
				}
			}
			for _, banned := range []string{"internal error", "NewFloat(NaN)", "comptime: "} {
				if strings.Contains(out, banned) {
					t.Errorf("unexpected %q in:\n%s", banned, out)
				}
			}
		})
	}
}

// Simple const Char and Bytes comparisons must agree with both ordinary
// expressions and comptime evaluation, including `!=` and the ends of the
// Char range.
func TestConstCharAndBytesComparisonsMatchRuntime(t *testing.T) {
	t.Parallel()
	src := `const CHAR_EQ: Bool = 'a' == 'a'
const CHAR_NE: Bool = 'a' != 'b'
const CHAR_LT: Bool = 'a' < 'z'
const CHAR_MIN_MAX: Bool = '\u{0}' < '\u{10FFFF}'
const BYTES_EQ: Bool = b"abc" == b"abc"
const BYTES_NE: Bool = b"abc" != b"abd"
pure fn charLt(a: Char, b: Char) -> Bool { a < b }
pure fn bytesEq(a: Bytes, b: Bytes) -> Bool { a == b }
const CT_CHAR: Bool = comptime charLt('a', 'z')
const CT_BYTES: Bool = comptime bytesEq(b"abc", b"abc")
print "${CHAR_EQ} ${CHAR_NE} ${CHAR_LT} ${CHAR_MIN_MAX} ${BYTES_EQ} ${BYTES_NE} ${CT_CHAR} ${CT_BYTES}"
let a: Char = 'a'
let z: Char = 'z'
let min: Char = '\u{0}'
let max: Char = '\u{10FFFF}'
print "${a == a} ${a != z} ${a < z} ${min < max}"
let abc: Bytes = b"abc"
let abd: Bytes = b"abd"
print "${abc == abc} ${abc != abd} ${min == '\u{0}'}"
`
	m := moduleOf(t, "cmp", map[string]string{"main.kg": src}, driver.LoadOptions{})
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, nil)
	if err != nil || code != 0 {
		t.Fatalf("vm: err %v exit %d\n%s", err, code, errOut.String())
	}
	want := "true true true true true true true true\n" +
		"true true true true\n" +
		"true true true\n"
	if got := out.String(); got != want {
		t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
