package vm

// startWorkers brings the pool's lanes up; lane 0 belongs to whichever thread
// drives the executor, and each worker polls lanes 1..n on its own Machine.
func (m *Machine) startWorkers(e *executor) {
	for i := 1; i <= e.workers; i++ {
		e.wg.Add(1)
		go m.lane().worker(e, i)
	}
}

func (m *Machine) worker(e *executor, lane int) {
	defer e.wg.Done()
	defer m.workerFailed(e)
	m.sh.sched.Lock()
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
		m.sh.sched.Unlock()
		m.runTask(t, lane)
		m.sh.sched.Lock()
		e.running--
		e.cv.Broadcast()
	}
	m.sh.sched.Unlock()
}

// workerFailed hands a worker's panic to the driving thread, which re-raises
// it, so a failure on any lane ends the program the same way.
func (m *Machine) workerFailed(e *executor) {
	r := recover()
	if r == nil {
		return
	}
	m.sh.sched.Lock()
	if e.failed == nil {
		e.failed = r
	}
	e.stopping = true
	e.cv.Broadcast()
	m.sh.sched.Unlock()
}

// closeExecutor runs from the Executor's drop, which a task on this very pool
// may be running; waiting there for the workers would then never return.
func (m *Machine) closeExecutor(e *executor) {
	if e.workers == 0 {
		return
	}
	m.sh.sched.Lock()
	e.dropped = true
	e.cv.Broadcast()
	idle := e.finished() && e.running == 0
	if !idle {
		m.sh.orphans = append(m.sh.orphans, e)
	}
	m.sh.sched.Unlock()
	if idle {
		e.wg.Wait()
	}
}

// Runs once the task tree is terminal, so every worker only has to leave its loop.
func (m *Machine) joinOrphans() {
	m.sh.sched.Lock()
	o := m.sh.orphans
	m.sh.orphans = nil
	m.sh.sched.Unlock()
	for _, e := range o {
		e.wg.Wait()
	}
}

// releaseLater defers a release taken under m.sh.sched: a drop hook runs
// program code, which the scheduler lock must not be held across.
func (m *Machine) releaseLater(v *obj) {
	if v != nil {
		m.pending = append(m.pending, v)
	}
}

func (m *Machine) flushReleases() {
	for len(m.pending) > 0 {
		p := m.pending
		m.pending = nil
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
		m.sh.sched.Lock()
		if ex.failed != nil {
			f := ex.failed
			m.sh.sched.Unlock()
			panic(f)
		}
		if root.terminal() {
			ex.driving = false
			m.sh.sched.Unlock()
			break
		}
		t := ex.take(0)
		if t != nil && t.terminal() {
			m.sh.sched.Unlock()
			continue
		}
		if t == nil {
			if ex.running > 0 || ex.external > 0 {
				ex.cv.Wait()
				m.sh.sched.Unlock()
				continue
			}
			m.sh.sched.Unlock()
			m.abort("every task on this executor is waiting for something that cannot happen")
		}
		ex.running++
		m.sh.sched.Unlock()
		m.runTask(t, 0)
		m.sh.sched.Lock()
		ex.running--
		m.sh.sched.Unlock()
	}
	return m.rootResult(root)
}

func (m *Machine) admitRoot(ex *executor, fut *obj) *task {
	m.sh.sched.Lock()
	if ex.driving {
		m.sh.sched.Unlock()
		m.abort("this executor is already running a task tree")
	}
	ex.driving = true
	root := &task{fut: fut, ex: ex}
	ex.live++
	ex.push(root)
	m.sh.sched.Unlock()
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
	m.sh.sched.Lock()
	m.endTask(root)
	m.sh.sched.Unlock()
	m.flushReleases()
	m.joinOrphans()
	return r
}
