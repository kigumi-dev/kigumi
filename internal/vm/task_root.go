package vm

func (m *Machine) runToCompletion(ex *executor, fut *obj) *obj {
	if m.cur != nil {
		m.abort("`Executor.runToCompletion` cannot run inside a task; `await` the future instead")
	}
	root := m.admitRoot(ex, fut)
	// Every exit but a completed root joins all pools first, held or orphaned.
	defer func() {
		if r := recover(); r != nil {
			m.drainPools()
			panic(r)
		}
	}()
	for {
		m.sh.sched.Lock()
		if ex.failed != nil {
			f := ex.failed
			m.sh.sched.Unlock()
			m.rethrow(f)
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
	if root.state == taskFaulted {
		root.fut = nil
	}
	r := root.result
	root.result = nil
	m.sh.sched.Lock()
	m.endTask(root)
	m.sh.sched.Unlock()
	m.flushReleases()
	m.joinOrphans()
	switch root.state {
	case taskCancelled:
		m.abort("the root task was cancelled")
	case taskFaulted:
		m.throw(root.fault)
	}
	return r
}
