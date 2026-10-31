package vm

// throw raises a copy of p, so the unwind owns the value it joins into.
func (m *Machine) throw(p *Panic) {
	m.closeOnPanic(p)
	c := *p
	m.inflight = &c
	panic(&c)
}

func (m *Machine) rethrow(r any) {
	if p, ok := r.(*Panic); ok {
		m.throw(p)
	}
	panic(r)
}

// joinInflight runs a deferred sweep on the panic in flight and folds any further panic into it in place.
// Recovering and panicking again at every level of a deep recursion costs time quadratic in its depth.
func (m *Machine) joinInflight(sweep func(p *Panic) *Panic) {
	own := m.inflight
	if own == nil || own.Sandbox || !m.containing() {
		return
	}
	if out := sweep(own); out != own {
		*own = *out
	}
	m.inflight = own
}
