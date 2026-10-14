package vm

import "sync/atomic"

// Inside a task a panic from one child's drop joins the others instead of ending the free.
func (m *Machine) releaseChild(c *obj, fault *Panic) *Panic {
	if m.cur == nil || c == nil || c.k < kRecord || atomic.LoadInt32(&c.rc) != 1 {
		m.release(c)
		return fault
	}
	return m.releaseJoined(c, fault)
}

func (m *Machine) runDropHook(v *obj, fault *Panic) (out *Panic) {
	drop := m.dropFn(v.ent)
	h, ent := m.hostDrop(v.ent)
	switch {
	case h != nil && m.hostWins(drop != nil):
		return m.callJoined(fault, func() { h(m, ent, []*obj{v}) })
	case drop != nil:
		return m.callJoined(fault, func() { m.invoke(drop, nil, false, []*obj{v}) })
	}
	return fault
}

func (m *Machine) callJoined(fault *Panic, run func()) (out *Panic) {
	if m.cur == nil {
		run()
		return fault
	}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		p, ok := r.(*Panic)
		if !ok || p.Sandbox {
			panic(r)
		}
		out = p
		if fault != nil {
			out = joinPanic(fault, p)
		}
	}()
	run()
	return fault
}
