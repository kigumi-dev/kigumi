package driver_test

import (
	"fmt"
	"strings"
	"testing"
)

const closureCrossSrc = `fn pick[T](v: T) -> T { v }

fn run(n: Int) -> Int {
    let k = n + 1
    let f = (x: Int) => x + k
    %s
}

print "ok=${run(4)}"
`

func TestFaultInjectionClosureBoxVM(t *testing.T) {
	t.Parallel()
	for _, call := range []string{"f(2)", "pick(f)(2)"} {
		if code, out, errOut := vmAtAlloc(t, fmt.Sprintf(closureCrossSrc, call), 1+interpTicks); code != 0 || out != "ok=7\n" {
			t.Errorf("%s: a direct closure counted as an allocation: exit %d stdout=%q stderr=%q", call, code, out, errOut)
		}
	}
	if code, _, errOut := vmAtFirstAlloc(t, literalPrint(fmt.Sprintf(closureCrossSrc, "let erased: fn(Int) -> Int = f\n    erased(2)"))); code != 2 || !strings.Contains(errOut, "out of memory") {
		t.Errorf("boxing a direct closure was not refused: exit %d stderr=%q", code, errOut)
	}
}
