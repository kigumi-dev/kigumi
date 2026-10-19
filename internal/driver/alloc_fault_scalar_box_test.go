package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/driver"
)

type scalarBoxCase struct {
	name, decls, fnType, cond, fnArg string
	perRound                         int
}

const scalarBoxTemplate = `%s

fn run(f: %s, n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        if %s { s = s + 1 }
    }
    s
}

print "ok=${run(%s, %d)}"
`

// A call through a function value hands a computed scalar over in a box each way unless the static tables hold it.
var scalarBoxCases = []scalarBoxCase{
	{"large int through a fn value", "fn big(x: Int) -> Int { x * 1000 + 7 }", "fn(Int) -> Int", "f(i + 2000) > 0", "big", 2},
	{"small int through a fn value", "fn small(x: Int) -> Int { x + 1 }", "fn(Int) -> Int", "f(i) > 0", "small", 0},
	{"float from a callback", "", "fn(Int) -> Float", "f(i) > 1.0", "(x: Int) => x.toFloat() * 1.5", 1},
	{"literal float from a callback", "", "fn(Int) -> Float", "f(i) > 1.0", "(x: Int) => 2.5", 0},
	{"char from a callback", "", "fn(Int) -> Char", "f(i + 3000) != 'a'", "(x: Int) => Char.fromInt(x + 10000) || 'a'", 2},
	{"ascii char from a callback", "", "fn(Int) -> Char", "f(i) == 'b'", "(x: Int) => if x > 1000 { 'a' } else { 'b' }", 0},
}

func (c scalarBoxCase) source(rounds int) string {
	return fmt.Sprintf(scalarBoxTemplate, c.decls, c.fnType, c.cond, c.fnArg, rounds)
}

func TestFaultInjectionScalarBoxesCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range scalarBoxCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			short, long := c.source(4), c.source(12)
			vmPer := vmAllocs(t, long) - vmAllocs(t, short)
			nativePer := nativeAllocs(t, long) - nativeAllocs(t, short)
			if vmPer != nativePer || nativePer != 8*c.perRound {
				t.Errorf("8 more rounds cost %d allocations on the VM and %d on native, want %d on both", vmPer, nativePer, 8*c.perRound)
			}
		})
	}
}
