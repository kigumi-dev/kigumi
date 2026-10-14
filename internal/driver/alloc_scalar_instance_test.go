package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/driver"
)

const scalarInstanceSrc = `fn wrap[T](x: T) -> T? { Some(x) }

fn wrapInt(x: Int) -> Int? { Some(x) }

fn firstOf[T: Copy](xs: &Array[T]) -> T? { xs.get(0) }

fn firstInt(xs: &Array[Int]) -> Int? { xs.get(0) }

fn run(rounds: Int) -> Int {
    let xs = Array.of(3, 4, 5)
    let mut s = 0
    for i in 0..rounds {
        s = s + (%s || 0)
    }
    s
}

print "ok=${run(%d)}"
`

// A scalar instantiation that returns an aggregate gets a body of its own, so it costs what a plain function does.
func TestScalarInstancesAllocateLikeAPlainFunction(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range []struct{ generic, plain string }{{"wrap(i)", "wrapInt(i)"}, {"firstOf(&xs)", "firstInt(&xs)"}} {
		t.Run(c.generic, func(t *testing.T) {
			t.Parallel()
			src := func(call string, rounds int) string { return fmt.Sprintf(scalarInstanceSrc, call, rounds) }
			native := func(call string) int { return nativeAllocs(t, src(call, 6)) - nativeAllocs(t, src(call, 2)) }
			vm := func(call string) int { return vmAllocs(t, src(call, 6)) - vmAllocs(t, src(call, 2)) }
			if g, p := native(c.generic), native(c.plain); g != p {
				t.Errorf("native: %s allocates %d more over 4 rounds, %s %d", c.generic, g, c.plain, p)
			}
			if g, p := vm(c.generic), vm(c.plain); g != p {
				t.Errorf("vm: %s allocates %d more over 4 rounds, %s %d", c.generic, g, c.plain, p)
			}
		})
	}
}

const scalarCallSrc = `noalloc fn keep[T: Copy](x: T) -> T { x }

noalloc fn keepInt(x: Int) -> Int { x + 1 }

noalloc fn pick[T: Copy](on: Bool, a: T, b: T) -> T { if on { a } else { b } }

noalloc fn half(x: Float) -> Float { x / 2.0 }

fn run(rounds: Int) -> Int {
    let mut s = 0
    for i in 0..rounds {
        s = s + %s
    }
    s
}

print "ok=${run(%d)}"
`

// A scalar crosses a call, generic or not, in a register: nothing is boxed beyond what the loop itself costs.
func TestScalarArgumentsCrossCallsUnboxed(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	src := func(expr string, rounds int) string { return fmt.Sprintf(scalarCallSrc, expr, rounds) }
	perRounds := func(count func(*testing.T, string) int, expr string) int {
		return count(t, src(expr, 6)) - count(t, src(expr, 2))
	}
	base := perRounds(nativeAllocs, "(i + 100000)")
	for _, expr := range []string{"keep(i + 100000)", "keepInt(i + 100000)", "pick(i > 2, i + 100000, i)", "(if half((i + 100000).toFloat()) > 1.0 { 1 } else { 0 })"} {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			if got := perRounds(nativeAllocs, expr); got != base {
				t.Errorf("native: %s allocates %d over 4 rounds, the bare loop %d", expr, got, base)
			}
			if got := perRounds(vmAllocs, expr); got != 0 {
				t.Errorf("vm: %s allocates %d over 4 rounds", expr, got)
			}
		})
	}
}
