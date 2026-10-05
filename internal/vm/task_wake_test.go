package vm

import "testing"

func queued(e *executor) int { return len(e.pool.q) + len(e.lane[0].q) }

// A producer wakes without knowing the task's state; several wakes before
// the task runs again must mean one more poll, not several.
func TestWakeCoalesces(t *testing.T) {
	m := &Machine{}
	ex := m.newExecutor(0)
	w := &task{ex: ex, state: taskSuspended}
	m.wake(w)
	m.wake(w)
	m.wake(w)
	if queued(ex) != 1 {
		t.Fatalf("three wakes queued the task %d times, want 1", queued(ex))
	}
	if got := ex.take(0); got != w || queued(ex) != 0 {
		t.Fatalf("take = %v with %d left, want the task and an empty queue", got, queued(ex))
	}
	w.state, w.notified = taskRunning, false
	m.wake(w)
	m.wake(w)
	if queued(ex) != 0 {
		t.Fatalf("a wake during a poll queued the task %d times, want 0", queued(ex))
	}
	if !w.notified {
		t.Fatal("a wake during a poll must stay sticky, so the poll's Pending re-queues the task")
	}
	w.state = taskCompleted
	m.wake(w)
	if queued(ex) != 0 {
		t.Fatalf("a terminal task was queued %d times, want 0", queued(ex))
	}
}

func TestCancelIsStickyAndReachesChildren(t *testing.T) {
	m := &Machine{}
	ex := m.newExecutor(0)
	parent := &task{ex: ex, state: taskSuspended}
	kid := &task{ex: ex, parent: parent, state: taskSuspended}
	parent.kids = []*task{kid}
	m.requestCancel(parent)
	m.requestCancel(parent)
	if !parent.cancelReq || !kid.cancelReq {
		t.Fatal("a cancel request must reach the task and its descendants")
	}
	if queued(ex) != 2 {
		t.Fatalf("the request queued %d tasks, want both woken once", queued(ex))
	}
	done := &task{ex: ex, state: taskCompleted}
	m.requestCancel(done)
	if done.cancelReq {
		t.Fatal("a request to a task that already finished must change nothing")
	}
}

// A pool keeps a pinned task on the lane that created it: the pool-wide
// queue a worker steals from must never hold one.
func TestPinnedTaskStaysOnItsLane(t *testing.T) {
	m := &Machine{}
	ex := m.newExecutor(2)
	bound := &task{ex: ex, lane: 2, state: taskSuspended}
	free := &task{ex: ex, lane: -1, state: taskSuspended}
	m.wake(bound)
	m.wake(free)
	if len(ex.pool.q) != 1 || ex.pool.q[0] != free {
		t.Fatalf("the pool queue holds %d tasks, want only the migratable one", len(ex.pool.q))
	}
	if len(ex.lane[2].q) != 1 || ex.lane[2].q[0] != bound {
		t.Fatalf("lane 2 holds %d tasks, want the pinned one", len(ex.lane[2].q))
	}
	if got := ex.take(1); got != free {
		t.Fatalf("lane 1 took %v, want the migratable task and never the pinned one", got)
	}
	if got := ex.take(1); got != nil {
		t.Fatalf("lane 1 took %v from another lane's queue", got)
	}
	if got := ex.take(2); got != bound {
		t.Fatalf("lane 2 took %v, want its own pinned task", got)
	}
}
