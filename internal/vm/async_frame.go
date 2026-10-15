package vm

import "kigumi/internal/mir"

// suspendPoint is one `await`: the operand moves into the frame so a
// resume re-polls it, and a cancellation request is observed first.
func (m *Machine) suspendPoint(fu *future, in *mir.Inst) (*obj, polled) {
	if fu.awaited == nil {
		fu.awaited = fu.co.fr.locals[in.Args[0]]
		fu.co.fr.locals[in.Args[0]] = nil
	}
	fu.at = in.Index
	if m.cur != nil && m.cur.cancelReq.Load() {
		m.dropFrame(fu)
		return nil, cancelled
	}
	v, st := m.pollValue(fu.awaited)
	if st != ready {
		if st == cancelled {
			m.dropFrame(fu)
		}
		return nil, st
	}
	m.release(fu.awaited)
	fu.awaited = nil
	return v, ready
}

// dropFrame releases the parked operand first, then locals in reverse slot order.
// Inside a task a drop that panics joins the fault and the rest still drop.
func (m *Machine) dropFrame(fu *future) {
	fu.done = true
	var fault *Panic
	if fu.awaited != nil {
		v := fu.awaited
		fu.awaited = nil
		fault = m.releaseJoined(v, fault)
	}
	co := fu.co
	if co == nil {
		m.raiseJoined(fault)
		return
	}
	fu.co = nil
	f := co.fr.f
	if fu.at < len(f.Cleanup) {
		for _, l := range f.Cleanup[fu.at] {
			v := co.fr.locals[l]
			co.fr.locals[l] = nil
			fault = m.releaseJoined(v, fault)
		}
	}
	m.raiseJoined(fault)
}

func (m *Machine) unwindFutures(root *obj) {
	var chain []*future
	for v := root; v != nil; {
		d := deref(v)
		fu, ok := d.opaqueData().(*future)
		if d.k != kOpaque || !ok || fu.co == nil {
			break
		}
		chain = append(chain, fu)
		v = fu.awaited
	}
	for i := len(chain) - 1; i >= 0; i-- {
		m.unwindCoro(chain[i])
	}
}

// Every local in the current resume is live, so sweep the frame, not the cleanup table.
func (m *Machine) unwindCoro(fu *future) {
	fu.done = true
	if fu.awaited != nil {
		v := fu.awaited
		fu.awaited = nil
		m.sweepRelease(v)
	}
	co := fu.co
	fu.co = nil
	m.dropOwnedLocals(co.fr.f, co.fr.locals)
}

func (m *Machine) freeFuture(fu *future) {
	m.dropFrame(fu)
	m.release(fu.fn)
	for _, a := range fu.args {
		m.release(a)
	}
	fu.fn, fu.args = nil, nil
	if fu.pool != nil {
		fu.pool.opaqueData().(*blockingPool).inFlight.Add(-1)
		m.release(fu.pool)
		fu.pool = nil
	}
	m.release(fu.result)
	fu.result = nil
}
