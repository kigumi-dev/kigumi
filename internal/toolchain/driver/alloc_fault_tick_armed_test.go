package driver_test

import (
	"slices"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const armedTickSweep = 32

func checkArmedTickParity(t *testing.T, m *driver.Module) {
	t.Helper()
	vmExits := vmSweepExitsTo(t, m, armedTickSweep, true)
	if vmExits[0] == 0 {
		t.Fatalf("vm: the first allocation after arming was not refused: %v", vmExits)
	}
	if vmExits[len(vmExits)-1] != 0 {
		t.Fatalf("vm still refuses at N=%d: %v", armedTickSweep, vmExits)
	}
	if native := nativeSweepExitsTo(t, m, armedTickSweep, true); !slices.Equal(vmExits, native) {
		t.Errorf("fault sweep exits differ\nvm:     %v\nnative: %v", vmExits, native)
	}
}
