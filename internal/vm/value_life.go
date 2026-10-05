package vm

// Mirrors the native rt_cellset.
func (m *Machine) cellSet(c *obj, v *obj) {
	if c == nil || c.k != kCell {
		m.abort("a `mut self` receiver reached the runtime without its cell")
	}
	old := c.inner
	c.inner = v
	m.release(old)
}

// release drops one reference; the machine runs the drop hook and frees
// the children when it was the last.
func (m *Machine) release(v *obj) {
	if v == nil || v.rc <= 0 {
		return
	}
	v.rc--
	if v.rc == 0 {
		m.free(v)
	}
}

// hostWins reports whether a std entity's host accelerator runs instead of its
// lowered body: always with no body, otherwise unless the accelerator is off.
func (m *Machine) hostWins(lowered bool) bool {
	return !lowered || !m.opts.NoAccel
}

func (m *Machine) free(v *obj) {
	m.unchargeMemory(v)
	switch v.k {
	case kRecord:
		v.rc = -1
		drop := m.dropFn(v.ent)
		if h, ent := m.hostDrop(v.ent); h != nil && m.hostWins(drop != nil) {
			h(m, ent, []*obj{v})
		} else if drop != nil {
			m.invoke(drop, nil, false, []*obj{v})
		}
		v.rc = 0
		for i := len(v.fields) - 1; i >= 0; i-- {
			m.release(v.fields[i])
		}
	case kVariant:
		for _, p := range v.fields {
			m.release(p)
		}
	case kArray:
		for i := len(v.fields) - 1; i >= 0; i-- {
			m.release(v.fields[i])
		}
	case kBox, kCell:
		m.release(v.inner)
	case kClosure:
		m.release(v.env)
	case kOpaque:
		m.freeOpaque(v)
	}
	// A freed object is poisoned so a later use is caught, like -DKIGUMI_RC_CHECK.
	*v = obj{k: kFree}
}

// copy duplicates a value the way rt_copy does: structural values deeply,
// resources and everything else by reference. Each level charges its own
// approxSize before returning, not only the outermost caller, since
// approxSize is shallow (a spine, not a subtree) (alloc_budget.go).
func (m *Machine) copy(v *obj) *obj {
	if v == nil {
		return v
	}
	switch v.k {
	case kRecord:
		if m.isResource(v.ent) {
			return retain(v)
		}
		c := mkRecord(v.ent, v.fields)
		for i := range c.fields {
			c.fields[i] = m.copy(c.fields[i])
		}
		m.chargeMemory(c)
		return c
	case kVariant:
		c := mkVariant(v.ent, v.fields)
		for i := range c.fields {
			c.fields[i] = m.copy(c.fields[i])
		}
		m.chargeMemory(c)
		return c
	case kArray:
		c := mkArray(v.fields)
		for i := range c.fields {
			c.fields[i] = m.copy(c.fields[i])
		}
		m.chargeMemory(c)
		return c
	case kBytes:
		// A fresh backing array, not just a retain: secureErase mutates
		// in place, and a shared slice would let it zero another copy too.
		c := mkBytes(append([]byte(nil), v.s...))
		m.chargeMemory(c)
		return c
	case kBox:
		c := newObj(kBox)
		c.dyn, c.inner = v.dyn, m.copy(v.inner)
		m.chargeMemory(c)
		return c
	}
	return retain(v)
}
