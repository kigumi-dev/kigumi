package vm

import "sync/atomic"

// Inside a task a panic from one child's drop joins the others instead of ending the free.
func (m *Machine) releaseChild(c *obj, fault *Panic) *Panic { return m.releaseChildAt(c, fault, 0) }

func (m *Machine) releaseChildAt(c *obj, fault *Panic, d int) *Panic {
	if !m.containing() || c == nil || c.k < kRecord || atomic.LoadInt32(&c.rc) != 1 {
		m.releaseAt(c, d)
		return fault
	}
	return m.callJoined(fault, func() { m.releaseAt(c, d) })
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
	if !m.containing() {
		run()
		return fault
	}
	base := len(m.frames)
	m.joinDepth++
	defer func() {
		m.joinDepth--
		if r := recover(); r != nil {
			out = m.recoverJoined(r, fault, base)
		}
	}()
	run()
	return fault
}
