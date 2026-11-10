package vm

// A spawned child inherits a cancellation its parent already carries.
func (m *Machine) spawnTask(fut *obj) *obj {
	if m.cur == nil {
		m.abort("`task.spawn` needs a running task; start one with `Executor.runToCompletion`")
	}
	m.checkOpen()
	return m.startTask(m.cur.ex, fut, m.futureMobile(fut))
}

// handoffTask is the explicit cross-executor move: the child stays a
// structured child of the running task but runs on the given executor.
func (m *Machine) handoffTask(ex *executor, fut *obj) *obj {
	if m.cur == nil {
		m.abort("`Executor.handoff` needs a running task; start one with `Executor.runToCompletion`")
	}
	m.checkOpen()
	if !m.futureMobile(fut) {
		m.abort("this future is bound to its own executor and cannot be handed to another one")
	}
	if ex != m.cur.ex && !m.executorHasAgent(ex) {
		m.abort("this executor has no worker and is not running a task tree, so it cannot take a handed-off task")
	}
	return m.startTask(ex, fut, true)
}

// executorHasAgent reads driving under the scheduler lock, since another lane sets it.
func (m *Machine) executorHasAgent(ex *executor) bool {
	m.sh.sched.Lock()
	defer m.sh.sched.Unlock()
	return ex.workers > 0 || ex.driving
}

// Admission is one linearization point: a child joins its parent's kid list
// and its executor's queue under the same lock.
func (m *Machine) startTask(ex *executor, fut *obj, mobile bool) *obj {
	m.allocHeader()
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
	p.addKid(t)
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
	st, fault := t.state, t.fault
	r := t.result
	t.result = nil
	m.forgetKid(t)
	m.sh.sched.Unlock()
	m.flushReleases()
	switch st {
	case taskCancelled:
		return nil, cancelled
	case taskFaulted:
		m.throw(fault)
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
	m.cancelKids(t)
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
	m.cancelKids(t)
	m.finishDrain(t)
}

func (m *Machine) finishDrain(t *task) {
	if t.openKids > 0 {
		t.state = taskSuspended
		return
	}
	m.inheritFault(t)
	t.state, t.draining = t.ending, false
	m.notifyScope(t)
	t.ex.live--
	if t.ex.finished() {
		t.ex.cv.Broadcast()
	}
	for _, k := range t.kids {
		if k != nil {
			m.endTask(k)
		}
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
	if t.parent != nil && (t.parent.draining || t.parent.waitKids) {
		m.wake(t.parent)
	}
	if t.detached && t.state != taskFaulted {
		m.forgetKid(t)
	}
}

// closeKids cancels the kids and reports whether any still runs; the last one to end wakes t.
func (m *Machine) closeKids(t *task) bool {
	m.sh.sched.Lock()
	defer m.sh.sched.Unlock()
	t.cancelReq.Store(true)
	t.closing = true
	m.cancelKids(t)
	open := t.openKids > 0
	t.waitKids = open
	return open
}

func (m *Machine) endTask(t *task) {
	f, r := t.fut, t.result
	t.fut, t.result = nil, nil
	m.releaseLater(f)
	m.releaseLater(r)
}

func (m *Machine) inheritFault(t *task) {
	if t.ending == taskFaulted {
		return
	}
	for _, k := range t.kids {
		if k != nil && k.state == taskFaulted {
			t.ending, t.fault = taskFaulted, k.fault
			return
		}
	}
}
