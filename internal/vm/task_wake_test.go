package vm

import "testing"

// A producer wakes without knowing the task's state; several wakes before
// the task runs again must mean one more poll, not several.
func TestWakeCoalesces(t *testing.T) {
	m := &Machine{}
	ex := &executor{}
	w := &task{ex: ex, state: taskSuspended}
	m.wake(w)
	m.wake(w)
	m.wake(w)
	if len(ex.queue) != 1 {
		t.Fatalf("three wakes queued the task %d times, want 1", len(ex.queue))
	}
	if got := ex.pop(); got != w || len(ex.queue) != 0 {
		t.Fatalf("pop = %v with %d left, want the task and an empty queue", got, len(ex.queue))
	}
	w.state, w.notified = taskRunning, false
	m.wake(w)
	m.wake(w)
	if len(ex.queue) != 0 {
		t.Fatalf("a wake during a poll queued the task %d times, want 0", len(ex.queue))
	}
	if !w.notified {
		t.Fatal("a wake during a poll must stay sticky, so the poll's Pending re-queues the task")
	}
	w.state = taskCompleted
	m.wake(w)
	if len(ex.queue) != 0 {
		t.Fatalf("a terminal task was queued %d times, want 0", len(ex.queue))
	}
}

func TestCancelIsStickyAndReachesChildren(t *testing.T) {
	m := &Machine{}
	ex := &executor{}
	parent := &task{ex: ex, state: taskSuspended}
	kid := &task{ex: ex, parent: parent, state: taskSuspended}
	parent.kids = []*task{kid}
	m.requestCancel(parent)
	m.requestCancel(parent)
	if !parent.cancelReq || !kid.cancelReq {
		t.Fatal("a cancel request must reach the task and its descendants")
	}
	if len(ex.queue) != 2 {
		t.Fatalf("the request queued %d tasks, want both woken once", len(ex.queue))
	}
	done := &task{ex: ex, state: taskCompleted}
	m.requestCancel(done)
	if done.cancelReq {
		t.Fatal("a request to a task that already finished must change nothing")
	}
}
