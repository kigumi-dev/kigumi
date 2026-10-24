package vm

// Runs after the callback's own frames are swept; a root task keeps its frames, so only the produced items go.
func (m *Machine) releaseProducedOnPanic(base int, done *bool, items func() []*obj) {
	if *done {
		return
	}
	m.joinInflight(func(p *Panic) *Panic {
		if m.sweepsPanics() {
			p = m.recoverJoined(p, nil, base)
		}
		produced := items()
		for i := len(produced) - 1; i >= 0; i-- {
			p = m.releaseJoined(produced[i], p)
		}
		return p
	})
}

func (m *Machine) tryMapItems(src, f *obj) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	var out []*obj
	done := false
	defer m.releaseProducedOnPanic(len(m.frames), &done, func() []*obj { return out })
	for _, x := range src.fields {
		out = append(out, m.callValueLent(f, []*obj{x}))
	}
	done = true
	return m.ok(m.sized(mkArray(out)))
}

func (m *Machine) tryFilterItems(src, keep *obj) *obj {
	if m.faultTick() {
		return m.allocError()
	}
	var out []*obj
	done := false
	defer m.releaseProducedOnPanic(len(m.frames), &done, func() []*obj { return out })
	for _, x := range src.fields {
		r := deref(m.callValueLent(keep, []*obj{x}))
		if r.b {
			out = append(out, m.copy(x))
		}
	}
	done = true
	return m.ok(m.sized(mkArray(out)))
}
