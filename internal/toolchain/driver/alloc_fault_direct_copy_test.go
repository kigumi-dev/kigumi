package driver_test

import (
	"slices"
	"testing"
)

func TestDirectRecordCopyDoesNotAllocate(t *testing.T) {
	m := loadSweepModule(t, `type Point = { n Int }
fn check(n: Int) {
    let a = Point { n: n }
    eprint "go"
    let b = a
    if a.n + b.n != 6 { let none: Int? = None; none.unwrap() }
    print "ok"
}
fn main() { check(3) }
`, "")
	vmExits := vmSweepExitsTo(t, m, 8, true)
	for i, code := range vmExits {
		if code != 0 {
			t.Errorf("vm N=%d: direct copy refused with exit %d", i+1, code)
		}
	}
	if native := nativeSweepExitsTo(t, m, 8, true); !slices.Equal(vmExits, native) {
		t.Errorf("fault sweep exits differ\nvm:     %v\nnative: %v", vmExits, native)
	}
}
