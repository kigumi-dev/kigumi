package vm

// A spawned child inherits a cancellation its parent already carries.
func (m *Machine) spawnTask(fut *obj) *obj {
	if m.cur == nil {
		m.abort("`task.spawn` needs a running task; start one with `Executor.runToCompletion`")
	}
	return m.startTask(m.cur.ex, fut, m.futureMobile(fut))
}

// handoffTask is the explicit cross-executor move: the child stays a
// structured child of the running task but runs on the given executor.
func (m *Machine) handoffTask(ex *executor, fut *obj) *obj {
	if m.cur == nil {
		m.abort("`Executor.handoff` needs a running task; start one with `Executor.runToCompletion`")
	}
	if !m.futureMobile(fut) {
		m.abort("this future is bound to its own executor and cannot be handed to another one")
	}
	if ex != m.cur.ex && ex.workers == 0 && !ex.driving {
		m.abort("this executor has no worker and is not running a task tree, so it cannot take a handed-off task")
	}
	return m.startTask(ex, fut, true)
}

// Admission is one linearization point: a child joins its parent's kid list
// and its executor's queue under the same lock.
func (m *Machine) startTask(ex *executor, fut *obj, mobile bool) *obj {
	m.sh.sched.Lock()
	p := m.cur
	t := &task{fut: fut, ex: ex, pex: p.ex, parent: p, lane: 0}
	t.cancelReq.Store(p.cancelReq.Load())
	if ex.workers > 0 {
		switch {
		case mobile:
			t.lane = -1
		case ex == p.ex:
			t.lane = m.curLane
		}
	}
	p.kids = append(p.kids, t)
	if ex != p.ex {
		p.ex.external++
	}
	ex.live++
	ex.push(t)
	m.sh.sched.Unlock()
	return mkOpaque("Task", t)
}

// Registers the waiter before reading target state, so a racing completion is still seen.
func (m *Machine) pollTask(t *task) (*obj, polled) {
	m.sh.sched.Lock()
	if !t.terminal() {
		t.waiter = m.cur
		m.sh.sched.Unlock()
		return nil, pending
	}
	t.waiter = nil
	st := t.state
	r := t.result
	t.result = nil
	m.sh.sched.Unlock()
	switch st {
	case taskCancelled:
		return nil, cancelled
	case taskFaulted:
		m.abort("the awaited task did not finish")
	}
	return r, ready
}

// Cancellation is cooperative: this only sets a sticky flag, observed at the next suspension point.
func (m *Machine) requestCancel(t *task) {
	m.sh.sched.Lock()
	m.cancelLocked(t)
	m.sh.sched.Unlock()
}

func (m *Machine) cancelLocked(t *task) {
	if t.terminal() || t.cancelReq.Load() {
		return
	}
	t.cancelReq.Store(true)
	for _, k := range t.kids {
		m.cancelLocked(k)
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
		m.cancelLocked(k)
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
	t.ex.live--
	if t.ex.finished() {
		t.ex.cv.Broadcast()
	}
	for _, k := range t.kids {
		m.endTask(k)
	}
	t.kids = nil
	if t.pex != nil && t.pex != t.ex {
		t.pex.external--
		t.pex.cv.Broadcast()
	}
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
	f, r := t.fut, t.result
	t.fut, t.result = nil, nil
	m.releaseLater(f)
	m.releaseLater(r)
}
