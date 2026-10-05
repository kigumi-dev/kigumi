package vm

// A spawned child inherits a cancellation its parent already carries.
func (m *Machine) spawnTask(fut *obj) *obj {
	if m.cur == nil {
		m.abort("`task.spawn` needs a running task; start one with `Executor.runToCompletion`")
	}
	t := &task{fut: fut, ex: m.cur.ex, parent: m.cur, cancelReq: m.cur.cancelReq}
	m.cur.kids = append(m.cur.kids, t)
	t.ex.push(t)
	return mkOpaque("Task", t)
}

// Registers the waiter before reading target state, so a racing completion is still seen.
func (m *Machine) pollTask(t *task) (*obj, polled) {
	if !t.terminal() {
		t.waiter = m.cur
		return nil, pending
	}
	t.waiter = nil
	switch t.state {
	case taskCancelled:
		return nil, cancelled
	case taskFaulted:
		m.abort("the awaited task did not finish")
	}
	r := t.result
	t.result = nil
	return r, ready
}

// Cancellation is cooperative: this only sets a sticky flag, observed at the next suspension point.
func (m *Machine) requestCancel(t *task) {
	if t.terminal() || t.cancelReq {
		return
	}
	t.cancelReq = true
	for _, k := range t.kids {
		m.requestCancel(k)
	}
	m.wake(t)
}

// wake coalesces: it only marks the task runnable, never runs its code.
func (m *Machine) wake(t *task) {
	if t.terminal() {
		return
	}
	t.notified = true
	if t.state == taskSuspended {
		t.ex.push(t)
	}
}

// beginFinish cancels still-running kids; the terminal transition waits for all of them.
func (m *Machine) beginFinish(t *task, end taskState) {
	t.ending, t.draining = end, true
	for _, k := range t.kids {
		m.requestCancel(k)
	}
	m.finishDrain(t)
}

func (m *Machine) finishDrain(t *task) {
	for _, k := range t.kids {
		if !k.terminal() {
			t.state = taskSuspended
			return
		}
	}
	t.state, t.draining = t.ending, false
	for _, k := range t.kids {
		m.endTask(k)
	}
	t.kids = nil
	if t.waiter != nil {
		w := t.waiter
		t.waiter = nil
		m.wake(w)
	}
	if t.parent != nil && t.parent.draining {
		m.wake(t.parent)
	}
}

func (m *Machine) endTask(t *task) {
	if t.fut != nil {
		f := t.fut
		t.fut = nil
		m.release(f)
	}
	if t.result != nil {
		r := t.result
		t.result = nil
		m.release(r)
	}
}
