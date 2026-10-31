package vm

import "testing"

func TestStrayFreeDoesNotCancelALeak(t *testing.T) {
	m := &Machine{sh: newShared()}
	outside := mkInt(100001, 0)
	TrackLive(true)
	t.Cleanup(func() { TrackLive(false) })
	leaked := mkInt(100002, 0)
	m.release(outside)
	if got := LiveObjects(); got != 1 {
		t.Errorf("live objects = %d, want the 1 leaked", got)
	}
	if got := LiveStrays(); got != 1 {
		t.Errorf("strays = %d, want 1", got)
	}
	m.release(leaked)
	if got := LiveObjects(); got != 0 {
		t.Errorf("live objects after the release = %d, want 0", got)
	}
}
