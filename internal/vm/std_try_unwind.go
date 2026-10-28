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

func (m *Machine) tryMapItems(key string, src, f *obj) *obj {
	n := int64(len(src.fields))
	if m.birthRefused(key, n) {
		return m.allocError()
	}
	var out []*obj
	done := false
	defer m.releaseProducedOnPanic(len(m.frames), &done, func() []*obj { return out })
	for _, x := range src.fields {
		out = append(out, m.callValueLent(f, []*obj{x}))
	}
	done = true
	return m.okTry(key, 1, m.sized(newArray(key, n, out)))
}

func (m *Machine) tryFilterItems(key string, src, keep *obj) *obj {
	if m.birthRefused(key, 0) {
		return m.allocError()
	}
	out := newArray(key, 0, nil)
	done := false
	defer m.releaseProducedOnPanic(len(m.frames), &done, func() []*obj { return out.fields })
	for _, x := range src.fields {
		if !deref(m.callValueLent(keep, []*obj{x})).b {
			continue
		}
		if !m.growFor(key, out, nil) {
			done = true
			m.release(out)
			return m.allocError()
		}
		out.fields = append(out.fields, m.copyPanicking(x))
	}
	done = true
	return m.okTry(key, 1, m.sized(out))
}
