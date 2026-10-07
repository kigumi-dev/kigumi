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
    %s(2)
}

print "ok=${run(4)}"
`

func TestFaultInjectionClosureBoxVM(t *testing.T) {
	t.Parallel()
	if code, out, errOut := vmAtFirstAlloc(t, fmt.Sprintf(closureCrossSrc, "f")); code != 0 || out != "ok=7\n" {
		t.Errorf("calling a direct closure counted as an allocation: exit %d stdout=%q stderr=%q", code, out, errOut)
	}
	if code, _, errOut := vmAtFirstAlloc(t, fmt.Sprintf(closureCrossSrc, "pick(f)")); code != 2 || !strings.Contains(errOut, "out of memory") {
		t.Errorf("boxing a direct closure was not refused: exit %d stderr=%q", code, errOut)
	}
}
