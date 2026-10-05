package vm

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
	fut       *obj
	result    *obj
	ex        *executor
	parent    *task
	waiter    *task
	kids      []*task
	state     taskState
	ending    taskState
	cancelReq bool
	notified  bool
	queued    bool
	draining  bool
}

// executor runs on the calling thread only; nothing here starts a thread of its own.
type executor struct {
	queue   []*task
	driving bool
}

func (t *task) terminal() bool {
	return t.state == taskCompleted || t.state == taskCancelled || t.state == taskFaulted
}

func (e *executor) push(t *task) {
	if t.queued {
		return
	}
	t.queued, t.state = true, taskRunnable
	e.queue = append(e.queue, t)
}

func (e *executor) pop() *task {
	if len(e.queue) == 0 {
		return nil
	}
	t := e.queue[0]
	e.queue = e.queue[1:]
	t.queued = false
	return t
}

func (m *Machine) runToCompletion(ex *executor, fut *obj) *obj {
	if m.cur != nil {
		m.abort("`Executor.runToCompletion` cannot run inside a task; `await` the future instead")
	}
	if ex.driving {
		m.abort("this executor is already running a task tree")
	}
	ex.driving = true
	defer func() { ex.driving = false }()
	root := &task{fut: fut, ex: ex}
	ex.push(root)
	for !root.terminal() {
		t := ex.pop()
		if t == nil {
			m.abort("every task on this executor is waiting for something that cannot happen")
		}
		m.runTask(t)
	}
	switch root.state {
	case taskCancelled:
		m.abort("the root task was cancelled")
	case taskFaulted:
		m.abort("the root task did not finish")
	}
	r := root.result
	root.result = nil
	m.endTask(root)
	return r
}

// A wake that arrives mid-poll is sticky: the task goes straight back on the queue.
func (m *Machine) runTask(t *task) {
	if t.terminal() {
		return
	}
	if t.draining {
		m.finishDrain(t)
		return
	}
	prev := m.cur
	m.cur, t.state, t.notified = t, taskRunning, false
	v, st := m.pollValue(t.fut)
	m.cur = prev
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
}
