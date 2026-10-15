package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/driver"
)

const rangeLoopSrc = `fn count(xs: &Array[Int], n: Int, small: u8) -> Int {
    let mut s = 0
    %s
    s
}

fn run(rounds: Int) -> Int {
    let xs = Array.of(1, 2, 3)
    let mut s = 0
    for i in 0..rounds {
        s = s + count(&xs, 3, 4)
    }
    s
}

print "ok=${run(%d)}"
`

var rangeLoopBodies = map[string]string{
	"exclusive": "for k in 0..n { s = s + k }",
	"inclusive": "for k in 0..=n { s = s + k }",
	"length":    "for k in 0..xs.len() { s = s + xs[k] }",
	"narrow":    "for k in 0..small { s = s + 1 }",
	"nested":    "for k in 0..n { for j in k..=n { s = s + j } }",
	"break":     "for k in 0..=n { if k == 2 { break }\n        s = s + k }",
	"wide":      "let hi: i128 = 4\n    for k in 0..hi { s = s + 1 }",
	"wideIncl":  "let hi: u128 = 4\n    for k in 0..=hi { s = s + 1 }",
}

// A counted loop over a written range keeps its bounds in registers: the callee allocates nothing per call.
func TestRangeLoopInCalleeAllocatesNothing(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for name, body := range rangeLoopBodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := func(rounds int) string { return fmt.Sprintf(rangeLoopSrc, body, rounds) }
			if got := nativeAllocs(t, src(6)) - nativeAllocs(t, src(2)); got != 0 {
				t.Errorf("native: %q allocates %d over 4 rounds", body, got)
			}
			if got := vmAllocs(t, src(6)) - vmAllocs(t, src(2)); got != 0 {
				t.Errorf("vm: %q allocates %d over 4 rounds", body, got)
			}
		})
	}
}

// A range that is a value of its own still allocates its record, so the loop reading it keeps iter.len and iter.at.
func TestRangeValueLoopStillBuildsTheRange(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	src := func(rounds int) string {
		return fmt.Sprintf(rangeLoopSrc, "let r = 0..n\n    for k in r { s = s + k }", rounds)
	}
	if got := nativeAllocs(t, src(6)) - nativeAllocs(t, src(2)); got <= 0 {
		t.Errorf("native: a range held in a local allocated %d over 4 rounds", got)
	}
}
