package vm

// Runs after the callback's own frames are swept, matching the native unwind order.
func (m *Machine) releaseProducedOnPanic(base int, items func() []*obj) {
	r := recover()
	if r == nil {
		return
	}
	p, ok := r.(*Panic)
	if !ok || p.Sandbox || !m.containing() {
		panic(r)
	}
	p = m.recoverJoined(r, nil, base)
	for _, x := range items() {
		p = m.releaseJoined(x, p)
	}
	panic(p)
}

func (m *Machine) tryMapItems(src, f *obj) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	var out []*obj
	defer m.releaseProducedOnPanic(len(m.frames), func() []*obj { return out })
	for _, x := range src.fields {
		out = append(out, m.callValue(f, []*obj{x}))
	}
	return m.ok(m.sized(mkArray(out)))
}

func (m *Machine) tryFilterItems(src, keep *obj) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	var out []*obj
	defer m.releaseProducedOnPanic(len(m.frames), func() []*obj { return out })
	for _, x := range src.fields {
		r := deref(m.callValue(keep, []*obj{x}))
		if r.b {
			out = append(out, retain(x))
		}
	}
	return m.ok(m.sized(mkArray(out)))
}
