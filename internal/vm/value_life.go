package vm

import "sync/atomic"

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
	if v == nil {
		return
	}
	if rc := atomic.LoadInt32(&v.rc); rc <= 0 {
		if rc == 0 && v.k == kFree {
			noteFreedRelease()
		}
		return
	}
	if atomic.AddInt32(&v.rc, -1) == 0 {
		m.free(v)
	}
}

// hostWins reports whether a std entity's host accelerator runs instead of its
// lowered body: always with no body, otherwise unless the accelerator is off.
func (m *Machine) hostWins(lowered bool) bool {
	return !lowered || !m.opts.NoAccel
}

func (m *Machine) free(v *obj) {
	switch v.k {
	case kInt, kFloat, kChar:
		tracked := v.live
		*v = obj{k: kFree, i: freedWord}
		if liveTrack.Load() {
			liveDropped(tracked)
			return
		}
		if len(m.pool) < scalarPoolMax {
			m.pool = append(m.pool, v)
		}
		return
	}
	m.unchargeMemory(v)
	var fault *Panic
	switch v.k {
	case kRecord:
		setImmortal(v)
		fault = m.runDropHook(v, nil)
		atomic.StoreInt32(&v.rc, 0)
		for i := len(v.fields) - 1; i >= 0; i-- {
			if !m.borrowField(v, i) {
				fault = m.releaseChild(v.fields[i], fault)
			}
		}
	case kVariant:
		for _, p := range v.fields {
			fault = m.releaseChild(p, fault)
		}
	case kArray:
		for i := len(v.fields) - 1; i >= 0; i-- {
			fault = m.releaseChild(v.fields[i], fault)
		}
	case kBox, kCell:
		if v.k != kCell || !v.b {
			fault = m.releaseChild(v.inner, fault)
		}
	case kClosure:
		fault = m.releaseChild(v.cenv(), fault)
	case kOpaque:
		m.freeOpaque(v)
	}
	dropExtension(v)
	// A freed object is poisoned so a later use is caught, like -DKIGUMI_RC_CHECK.
	tracked := v.live
	*v = obj{k: kFree}
	if liveTrack.Load() {
		liveDropped(tracked)
	}
	m.raiseJoined(fault)
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
			if !m.borrowField(v, i) {
				c.fields[i] = m.copy(c.fields[i])
			}
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
		c := mkBytes(append([]byte(nil), v.str()...))
		m.chargeMemory(c)
		return c
	case kBox:
		c := newObj(kBox)
		c.i, c.inner = v.i, m.copy(v.inner)
		m.chargeMemory(c)
		return c
	}
	return retain(v)
}

// writableInside mirrors the native rt_copy_mut's test.
func (m *Machine) writableInside(v *obj) bool {
	if v == nil {
		return false
	}
	switch v.k {
	case kArray:
		return true
	case kRecord:
		if m.r.RecordWritable(v.ent) {
			return true
		}
		if m.isResource(v.ent) {
			return false
		}
		for i, f := range v.fields {
			if !m.borrowField(v, i) && m.writableInside(f) {
				return true
			}
		}
	case kVariant:
		for _, f := range v.fields {
			if m.writableInside(f) {
				return true
			}
		}
	}
	return false
}
