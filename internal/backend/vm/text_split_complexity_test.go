package vm_test

import (
	"testing"

	"kigumi/internal/backend/vm"
)

func TestSplitSearchStaysWithinLinearBudget(t *testing.T) {
	_, stderr, code := buildAndRun(t, `
import test from std/test
let s = "a".repeat(4000)
let sep = "${"a".repeat(2000)}b"
let parts = s.split(sep)
test.assert(parts.len() == 1, "unexpected split")
`, vm.Options{Sandbox: true, NoAccel: true, StepLimit: 8000000, DepthLimit: 1000})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}
