package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

const printfLiterals = `
extern(C) {
    fn printf(fmt: *const u8, ...) -> i32
}

fn small(fmt: *const u8, s: *const u8) {
    unsafe { printf(fmt, 42, s) }
}

fn wide(fmt: *const u8, s: *const u8) {
    unsafe { printf(fmt, 5000000000, s) }
}

fn typed(fmt: *const u8, s: *const u8, n: i64) {
    unsafe { printf(fmt, n, s) }
}

fn main() -> Void! {
}
`

// An unsuffixed integer literal in a C variadic position is a C int when it fits, so printf("%d %s", 42, p) reads the same 32 bits on every target.
func TestCVariadicLiteralDefaultsToCInt(t *testing.T) {
	ir := emitIR(t, printfLiterals)
	call := regexp.MustCompile(`call i32 \(ptr, \.\.\.\) @printf\(ptr [^,]+, (i\d+) [^,]+, ptr [^)]+\)`)
	for fn, want := range map[string]string{"main.small": "i32", "main.wide": "i64", "main.typed": "i64"} {
		m := call.FindStringSubmatch(fnBody(t, ir, fn))
		if m == nil {
			t.Errorf("%s: no printf call in:\n%s", fn, fnBody(t, ir, fn))
			continue
		}
		if m[1] != want {
			t.Errorf("%s: the variadic integer is %s, want %s", fn, m[1], want)
		}
	}
	if !strings.Contains(ir, "declare i32 @printf(ptr, ...)") {
		t.Errorf("printf is not declared variadic:\n%s", ir)
	}
}
