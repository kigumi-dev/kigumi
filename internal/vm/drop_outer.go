package vm

// Like native, only where the build can contain a drop panic; elsewhere a drop outside a task ends it at once.
func (m *Machine) dropChecked(fr *frame, v *obj) {
	if m.cur != nil || m.outerDepth > 0 || v == nil || m.opts.Sandbox || !m.p.ContainsDropPanics() {
		m.release(v)
		return
	}
	base := len(m.frames)
	m.outerDepth++
	defer func() {
		defer func() { m.outerDepth-- }()
		if r := recover(); r != nil {
			p, ok := r.(*Panic)
			if !ok || p.Sandbox {
				panic(r)
			}
			panic(m.sweepOuter(fr, base, p))
		}
	}()
	m.release(v)
}

func (m *Machine) sweepOuter(fr *frame, base int, fault *Panic) *Panic {
	outer := m.sweepFault
	m.sweepFault = fault
	m.sweepFrames(base)
	for _, l := range m.unwindSet(fr) {
		v := fr.locals[l]
		if v == nil || m.ownsNothing(fr.f.Locals[l].Type) {
			continue
		}
		fr.locals[l] = nil
		m.sweepRelease(v)
	}
	out := m.sweepFault
	m.sweepFault = outer
	return out
}
