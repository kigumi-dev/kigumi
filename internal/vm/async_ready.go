package vm

// takeReady completes a BlockingPool.run Future: its result was computed
// eagerly at run time, so the first poll only releases the pool's admission.
func (m *Machine) takeReady(fu *future) *obj {
	fu.done = true
	m.dropAdmission(fu)
	r := fu.result
	fu.result = nil
	return r
}

// pollYield is pending once with the task marked runnable, so it goes to the back of its queue behind its siblings.
func (m *Machine) pollYield(fu *future) (*obj, polled) {
	if fu.yield == 1 && m.cur != nil {
		fu.yield = 2
		m.sh.sched.Lock()
		m.cur.notified = true
		m.sh.sched.Unlock()
		return nil, pending
	}
	fu.done = true
	return unitObj, ready
}
