package vm

import (
	"strings"
	"testing"
)

func TestStaleReleaseOnTheUnwindPathIsCounted(t *testing.T) {
	m := &Machine{sh: newShared()}
	TrackLive(true)
	t.Cleanup(func() { TrackLive(false) })
	v := mkInt(100001, 0)
	stale := v
	m.sweepRelease(v)
	m.sweepRelease(stale)
	if got := liveFreedUse.Load(); got != 1 {
		t.Errorf("releases of a freed scalar = %d, want 1", got)
	}
	if again := m.newInt(100002, 0); again == stale {
		t.Errorf("a freed scalar was recycled while tracking, so a stale pointer would alias a live value")
	}
}

func TestWriteThroughAStalePointerIsSeenOnReuse(t *testing.T) {
	m := &Machine{sh: newShared()}
	v := mkInt(100001, 0)
	stale := v
	m.sweepRelease(v)
	if len(m.pool) != 1 {
		t.Fatalf("pool = %d, want the freed scalar", len(m.pool))
	}
	stale.i = 7
	defer func() {
		p, ok := recover().(*Panic)
		if !ok || !strings.Contains(p.Msg, "used after free") {
			t.Errorf("recover = %v, want the use-after-free abort", p)
		}
	}()
	m.newInt(100003, 0)
}
