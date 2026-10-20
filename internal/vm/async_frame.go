package vm

import "kigumi/internal/mir"

func (m *Machine) stepAsync(fu *future, in *mir.Inst) (st polled, stop bool) {
	co := fu.co
	switch in.Op {
	case mir.OpAwait:
		co.fr.blk, co.fr.pc = co.bid, co.pc
		v, st := m.suspendPoint(fu, in)
		if st == pending {
			return st, true
		}
		co.fr.locals[in.Dst] = v
		co.cancelled = st == cancelled
	case mir.OpCancelled:
		co.fr.locals[in.Dst] = mkBool(co.cancelled)
		co.cancelled = false
	default:
		fu.done, fu.co = true, nil
		return cancelled, true
	}
	co.pc++
	return ready, false
}

// suspendPoint is one `await`: the operand moves into the frame so a
// resume re-polls it, and a cancellation request is observed first.
func (m *Machine) suspendPoint(fu *future, in *mir.Inst) (*obj, polled) {
	if fu.awaited == nil {
		fu.awaited = fu.co.fr.locals[in.Args[0]]
		fu.co.fr.locals[in.Args[0]] = nil
	}
	fu.at = in.Index
	if fu.forceCancel {
		return nil, m.cancelAwait(fu)
	}
	if m.cur != nil && m.cur.cancelReq.Load() {
		return m.cancelWhenKidsClosed(fu)
	}
	v, st := m.pollValue(fu.awaited)
	switch st {
	case pending:
		return nil, st
	case cancelled:
		return m.cancelWhenKidsClosed(fu)
	}
	m.release(fu.awaited)
	fu.awaited = nil
	return v, ready
}

func (m *Machine) cancelWhenKidsClosed(fu *future) (*obj, polled) {
	if m.cur != nil && m.closeKids(m.cur) {
		return nil, pending
	}
	return nil, m.cancelAwait(fu)
}

func (m *Machine) cancelAwait(fu *future) polled {
	var fault *Panic
	if v := fu.awaited; v != nil {
		fu.awaited = nil
		fault = m.releaseJoined(v, fault)
	}
	m.raiseJoined(fault)
	return cancelled
}

// A panic in its cleanups sweeps the rest of the body here: no task chain reaches it any more.
func (m *Machine) cancelSuspended(fu *future) {
	fu.forceCancel = true
	base := len(m.frames)
	defer func() {
		if r := recover(); r != nil {
			panic(m.sweepCoro(fu, m.recoverJoined(r, nil, base)))
		}
	}()
	if _, st := m.resume(fu); st != cancelled {
		m.abort("a suspended Future did not finish as cancelled")
	}
}

func (m *Machine) sweepCoro(fu *future, fault *Panic) *Panic {
	outer := m.sweepFault
	m.sweepFault = fault
	if fu.co != nil {
		m.unwindCoro(fu)
	}
	out := m.sweepFault
	m.sweepFault = outer
	return out
}

// dropFrame releases the parked operand first, then locals in reverse slot order.
// Inside a task a drop that panics joins the fault and the rest still drop.
func (m *Machine) dropFrame(fu *future) {
	if fu.co != nil && m.cur != nil && m.sweepFault == nil && !fu.forceCancel {
		m.cancelSuspended(fu)
		return
	}
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
