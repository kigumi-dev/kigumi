package vm

// callStdValue is rt_callv on a std function value; a panic in the primitive still releases the arguments it does not borrow.
func (m *Machine) callStdValue(fv *obj, args []*obj) *obj {
	all := append(append([]*obj{}, fv.cenv().fields...), args...)
	var rec *frame
	if m.sweepsPanics() {
		rec = m.heldFrame()
		for i := range args {
			if fv.clent()>>i&1 == 0 {
				rec.held = append(rec.held, args[i])
			}
		}
	}
	// A primitive whose meaning depends on its declaration (a numeric
	// conversion) is dispatched as if called directly.
	site, siteFn := m.site, m.siteFn
	m.site, m.siteFn = m.valueSite(fv, len(fv.cenv().fields))
	h, _ := m.hostFor(fv.cprov(), fv.cstd())
	r := m.callPrim(fv.ent, fv.cstd(), h, all)
	m.site, m.siteFn = site, siteFn
	if !stdHash(fv) {
		m.scalarTick(r)
	}
	if rec != nil {
		m.raiseJoined(m.settleInflight(rec))
		return r
	}
	// A bound primitive lends its receiver and borrowed parameters
	// (rt_std_bind).
	for i, a := range args {
		if fv.clent()>>i&1 == 0 {
			m.release(a)
		}
	}
	return r
}
