package vm

// minArrayCap is the smallest backing store native gives a non-empty array literal.
const minArrayCap = 4

func mkArray(items []*obj) *obj {
	v := withFields(kArray, 0, items)
	if n := int64(len(items)); n > 0 {
		v.i = max(n, minArrayCap)
	}
	return v
}

// reserveArray mirrors native try_grow's capacity rule and reports whether the backing store has to be reallocated.
func reserveArray(a *obj, extra uint64) bool {
	need := uint64(len(a.fields)) + extra
	capacity := uint64(max(a.i, int64(len(a.fields))))
	if need <= capacity {
		a.i = int64(capacity)
		return false
	}
	a.i = int64(max(capacity*2, need))
	return true
}

// growArray takes native's refusable allocation, only when the modelled capacity is exceeded.
func (m *Machine) growArray(a *obj, extra uint64) bool {
	if !reserveArray(a, extra) {
		return true
	}
	if m.faultTick() {
		return false
	}
	reserveArray(a, extra)
	return true
}

// userCaller: std bodies over these primitives are ticked per body to match native's intrinsics, so only user code gets the per-call model.
func (m *Machine) userCaller() bool {
	if len(m.frames) == 0 {
		return true
	}
	f := m.frames[len(m.frames)-1].f
	return f == nil || f.Ent == 0 || !m.r.Packages[m.r.Entity(f.Ent).Pkg].Std
}

func (m *Machine) growBy(a *obj, extra uint64) bool {
	if m.userCaller() {
		return m.growArray(a, extra)
	}
	if m.faultTick() {
		return false
	}
	reserveArray(a, extra)
	return true
}
