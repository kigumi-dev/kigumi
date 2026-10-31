package vm

import (
	"testing"
	"time"
)

func within(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatalf("%s did not finish", what)
	}
}

// A task running on the pool may be the one dropping its handle, so the drop
// only asks the workers to stop and leaves the wait to the driving thread.
func TestDropWithALiveTaskDefersTheJoin(t *testing.T) {
	m := &Machine{sh: newShared()}
	ex := m.newExecutor(1)
	m.startWorkers(ex)
	m.sh.sched.Lock()
	ex.live++
	m.sh.sched.Unlock()

	within(t, "the drop", func() { m.closeExecutor(ex) })
	m.sh.sched.Lock()
	if len(m.sh.orphans) != 1 || m.sh.orphans[0] != ex {
		t.Fatalf("the drop left %d pools for the join, want the one it could not wait for", len(m.sh.orphans))
	}
	ex.live--
	ex.cv.Broadcast()
	m.sh.sched.Unlock()

	within(t, "the join", m.joinOrphans)
	if len(m.sh.orphans) != 0 {
		t.Fatalf("%d pools are still waiting to be joined", len(m.sh.orphans))
	}
}

// A pool nothing can reach any more is joined by the drop itself, so a
// program that makes one per round does not pile workers up.
func TestDropWithNoTaskJoinsRightAway(t *testing.T) {
	m := &Machine{sh: newShared()}
	ex := m.newExecutor(2)
	m.startWorkers(ex)
	within(t, "the drop", func() { m.closeExecutor(ex) })
	if len(m.sh.orphans) != 0 {
		t.Fatalf("an idle pool was left for the join")
	}
}
