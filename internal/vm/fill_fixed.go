package vm

func (m *Machine) fillFixed(key string, value *obj, n int64) *obj {
	inline := m.inlineResultCopy(key)
	if !inline && m.birthRefused(key, 0) {
		m.abort("out of memory")
	}
	out := newArray(key, 0, nil)
	done := false
	defer m.releaseProducedOnPanic(len(m.frames), &done, func() []*obj { return []*obj{out} })
	for range n {
		item := m.copiedResult(key, value)
		if !inline && !m.growFor(key, out, nil) {
			m.release(item)
			m.abort("out of memory")
		}
		out.fields = append(out.fields, item)
	}
	m.chargeMemory(out)
	done = true
	return out
}
