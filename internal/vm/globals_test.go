package vm

import (
	"testing"

	"kigumi/internal/sem"
)

// A static mut's old value must be released like OpSetField's, not just overwritten.
func TestStoreGlobalReleasesPreviousValue(t *testing.T) {
	m := &Machine{sh: newShared()}
	const ent sem.EntityID = 1
	old := mkInt(1, 0)
	m.sh.globals[ent] = old
	next := mkInt(2, 0)

	m.storeGlobal(ent, next)

	if old.k != kFree {
		t.Fatalf("previous global value not released: k = %v, want kFree", old.k)
	}
	if got := m.sh.globals[ent]; got != next {
		t.Fatalf("storeGlobal did not install the new value")
	}
}
