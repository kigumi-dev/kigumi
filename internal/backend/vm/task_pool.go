package vm

import "slices"

// startWorkers brings the pool's lanes up; lane 0 belongs to whichever thread
// drives the executor, and each worker polls lanes 1..n on its own Machine.
func (m *Machine) startWorkers(e *executor) {
	m.sh.sched.Lock()
	m.sh.pools = append(m.sh.pools, e)
	m.sh.sched.Unlock()
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
		m.forgetPool(e)
	}
}

func (m *Machine) forgetPool(e *executor) {
	m.sh.sched.Lock()
	m.sh.pools = slices.DeleteFunc(m.sh.pools, func(p *executor) bool { return p == e })
	m.sh.sched.Unlock()
}

// Runs once the task tree is terminal, so every worker only has to leave its loop.
func (m *Machine) joinOrphans() {
	m.sh.sched.Lock()
	o := m.sh.orphans
	m.sh.orphans = nil
	m.sh.sched.Unlock()
	for _, e := range o {
		e.wg.Wait()
		m.forgetPool(e)
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

// stopPools runs no drop code and does not wait: a timed-out test has no one to wait for.
func (m *Machine) stopPools() []*executor {
	m.sh.sched.Lock()
	defer m.sh.sched.Unlock()
	pools := slices.Clone(m.sh.pools)
	for _, e := range pools {
		e.stopping = true
		e.cv.Broadcast()
	}
	return pools
}

func (m *Machine) drainPools() {
	for _, e := range m.stopPools() {
		e.wg.Wait()
		m.forgetPool(e)
	}
}
