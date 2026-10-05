package vm

// startWorkers brings the pool's lanes up; lane 0 belongs to whichever
// thread drives the executor, so the workers take lanes 1..n.
func (m *Machine) startWorkers(e *executor) {
	for i := 1; i <= e.workers; i++ {
		e.wg.Add(1)
		go m.worker(e, i)
	}
}

func (m *Machine) worker(e *executor, lane int) {
	defer e.wg.Done()
	defer m.workerFailed(e)
	m.sched.Lock()
	for {
		t := e.take(lane)
		if t == nil {
			if e.finished() {
				break
			}
			e.cv.Wait()
			continue
		}
		// A terminal task needs no body, so taking one never blocks on run.
		if t.terminal() {
			continue
		}
		e.running++
		m.sched.Unlock()
		m.runTask(t, lane)
		m.sched.Lock()
		e.running--
		e.cv.Broadcast()
	}
	m.sched.Unlock()
}

// workerFailed hands a worker's panic to the driving thread, which re-raises
// it, so a failure on any lane ends the program the same way.
func (m *Machine) workerFailed(e *executor) {
	r := recover()
	if r == nil {
		return
	}
	m.sched.Lock()
	if e.failed == nil {
		e.failed = r
	}
	e.stopping = true
	e.cv.Broadcast()
	m.sched.Unlock()
}

// closeExecutor runs from the Executor's drop, which may hold run; waiting
// there for a worker that still owes a task would never return.
func (m *Machine) closeExecutor(e *executor) {
	if e.workers == 0 {
		return
	}
	m.sched.Lock()
	e.dropped = true
	e.cv.Broadcast()
	idle := e.finished() && e.running == 0
	if !idle {
		m.orphans = append(m.orphans, e)
	}
	m.sched.Unlock()
	if idle {
		e.wg.Wait()
	}
}

// Runs once the task tree is terminal, so every worker only has to leave its loop.
func (m *Machine) joinOrphans() {
	m.sched.Lock()
	o := m.orphans
	m.orphans = nil
	m.sched.Unlock()
	for _, e := range o {
		e.wg.Wait()
	}
}

// releaseLater defers a release taken under m.sched: a drop hook runs
// program code, which the scheduler lock must not be held across.
func (m *Machine) releaseLater(v *obj) {
	if v != nil {
		m.pending = append(m.pending, v)
	}
}

func (m *Machine) flushReleases() {
	for {
		m.sched.Lock()
		p := m.pending
		m.pending = nil
		m.sched.Unlock()
		if len(p) == 0 {
			return
		}
		for _, v := range p {
			m.release(v)
		}
	}
}

func (m *Machine) runToCompletion(ex *executor, fut *obj) *obj {
	if m.cur != nil {
		m.abort("`Executor.runToCompletion` cannot run inside a task; `await` the future instead")
	}
	root := m.admitRoot(ex, fut)
	for {
		m.sched.Lock()
		if ex.failed != nil {
			f := ex.failed
			m.sched.Unlock()
			panic(f)
		}
		if root.terminal() {
			ex.driving = false
			m.sched.Unlock()
			break
		}
		t := ex.take(0)
		if t != nil && t.terminal() {
			m.sched.Unlock()
			continue
		}
		if t == nil {
			if ex.running > 0 || ex.external > 0 {
				ex.cv.Wait()
				m.sched.Unlock()
				continue
			}
			m.sched.Unlock()
			m.abort("every task on this executor is waiting for something that cannot happen")
		}
		ex.running++
		m.sched.Unlock()
		m.runTask(t, 0)
		m.sched.Lock()
		ex.running--
		m.sched.Unlock()
	}
	return m.rootResult(root)
}

func (m *Machine) admitRoot(ex *executor, fut *obj) *task {
	m.sched.Lock()
	if ex.driving {
		m.sched.Unlock()
		m.abort("this executor is already running a task tree")
	}
	ex.driving = true
	root := &task{fut: fut, ex: ex}
	ex.live++
	ex.push(root)
	m.sched.Unlock()
	return root
}

func (m *Machine) rootResult(root *task) *obj {
	switch root.state {
	case taskCancelled:
		m.abort("the root task was cancelled")
	case taskFaulted:
		m.abort("the root task did not finish")
	}
	r := root.result
	root.result = nil
	m.run.Lock()
	m.sched.Lock()
	m.endTask(root)
	m.sched.Unlock()
	m.flushReleases()
	m.run.Unlock()
	m.joinOrphans()
	return r
}
