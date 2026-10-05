package vm

import "sync"

type taskState uint8

const (
	taskCreated taskState = iota
	taskRunnable
	taskRunning
	taskSuspended
	taskCompleted
	taskCancelled
	taskFaulted
)

type task struct {
	fut    *obj
	result *obj
	ex     *executor
	// pex is the executor the parent runs on; it differs from ex only for a
	// handed-off child, whose parent must not be declared stuck while it runs.
	pex       *executor
	parent    *task
	waiter    *task
	kids      []*task
	state     taskState
	ending    taskState
	lane      int
	cancelReq bool
	notified  bool
	queued    bool
	draining  bool
}

type taskQueue struct{ q []*task }

func (k *taskQueue) push(t *task) { k.q = append(k.q, t) }

func (k *taskQueue) pop() *task {
	if len(k.q) == 0 {
		return nil
	}
	t := k.q[0]
	k.q = k.q[1:]
	return t
}

// executor owns one pool-wide queue for migratable tasks and one queue per
// lane for the tasks pinned to it; lane 0 is the thread driving it.
type executor struct {
	pool    taskQueue
	lane    []taskQueue
	cv      *sync.Cond
	wg      sync.WaitGroup
	driving bool
	// dropped is set by the handle's drop, stopping by a worker panic; live
	// counts the tasks admitted here that have not reached a terminal state.
	dropped  bool
	stopping bool
	live     int
	running  int
	external int
	workers  int
	// failed carries a worker's panic to the thread driving the executor.
	failed any
}

// A worker with an empty queue may leave only once the handle is dropped and
// nothing it admitted is still live. Read under sched.
func (e *executor) finished() bool { return e.stopping || (e.dropped && e.live == 0) }

func (m *Machine) newExecutor(workers int) *executor {
	e := &executor{lane: make([]taskQueue, workers+1), workers: workers, cv: sync.NewCond(&m.sched)}
	return e
}

func (t *task) terminal() bool {
	return t.state == taskCompleted || t.state == taskCancelled || t.state == taskFaulted
}

// push and take run under m.sched; work stealing never reaches a lane queue.
func (e *executor) push(t *task) {
	if t.queued {
		return
	}
	t.queued, t.state = true, taskRunnable
	if t.lane < 0 {
		e.pool.push(t)
	} else {
		e.lane[t.lane].push(t)
	}
	e.cv.Broadcast()
}

func (e *executor) take(lane int) *task {
	t := e.lane[lane].pop()
	if t == nil {
		t = e.pool.pop()
	}
	if t != nil {
		t.queued = false
	}
	return t
}

// A wake that arrives mid-poll is sticky: the task goes straight back on the queue.
func (m *Machine) runTask(t *task, lane int) {
	m.run.Lock()
	defer m.run.Unlock()
	m.sched.Lock()
	if t.terminal() {
		m.sched.Unlock()
		return
	}
	if t.draining {
		m.finishDrain(t)
		m.sched.Unlock()
		m.flushReleases()
		return
	}
	t.state, t.notified = taskRunning, false
	m.sched.Unlock()
	prev, prevLane := m.cur, m.curLane
	m.cur, m.curLane = t, lane
	v, st := m.pollValue(t.fut)
	m.cur, m.curLane = prev, prevLane
	m.sched.Lock()
	switch st {
	case ready:
		t.result = v
		m.beginFinish(t, taskCompleted)
	case cancelled:
		m.beginFinish(t, taskCancelled)
	default:
		t.state = taskSuspended
		if t.notified {
			t.ex.push(t)
		}
	}
	m.sched.Unlock()
	m.flushReleases()
}
