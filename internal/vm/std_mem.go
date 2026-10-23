package vm

// memStd implements std/mem: UnsafeCell and Uninit are opaque handles around a
// plain VM cell; an Uninit's cell holds nil while empty.
func (m *Machine) memStd(key string, a []*obj, a0, a1 *obj) *obj {
	switch key {
	case "mem.UnsafeCell.new":
		return mkOpaque("UnsafeCell", mkCell(retain(a0)))
	case "mem.UnsafeCell.get":
		return m.copy(a0.opaqueData().(*obj).inner)
	case "mem.UnsafeCell.set":
		cell := a0.opaqueData().(*obj)
		old := cell.inner
		cell.inner = retain(a1)
		m.release(old)
		return unitObj
	case "mem.UnsafeCell.atomicLoad", "mem.UnsafeCell.atomicStore", "mem.UnsafeCell.atomicExchange", "mem.UnsafeCell.atomicCompareExchange",
		"mem.UnsafeCell.atomicAdd", "mem.UnsafeCell.atomicSub", "mem.UnsafeCell.atomicAnd", "mem.UnsafeCell.atomicOr", "mem.UnsafeCell.atomicXor":
		return m.atomicCell(key, a0, a[1:len(a)-1])
	case "mem.uninit":
		return mkOpaque("Uninit", mkCell(nil))
	case "mem.initialize":
		a0.opaqueData().(*obj).inner = retain(a1)
		return unitObj
	case "mem.borrowInitialized":
		return retain(m.filled(a0))
	case "mem.copyInitialized":
		return m.copy(m.filled(a0))
	case "mem.borrowInitializedMut":
		return retain(a0.opaqueData().(*obj))
	case "mem.takeInitialized":
		cell := a0.opaqueData().(*obj)
		v := m.filled(a0)
		cell.inner = nil
		return v
	case "mem.dropInitialized":
		cell := a0.opaqueData().(*obj)
		old := cell.inner
		cell.inner = nil
		m.release(old)
		return unitObj
	case "mem.secureErase":
		for i := range a0.str() {
			a0.str()[i] = 0
		}
		return unitObj
	case "mem.trySecureErase":
		for i := range a0.str() {
			a0.str()[i] = 0
		}
		return m.ok(unitObj)
	}
	return nil
}

// filled is the content of an Uninit; reading an empty one (an element lost to a panic) aborts.
func (m *Machine) filled(u *obj) *obj {
	v := u.opaqueData().(*obj).inner
	if v == nil {
		m.abort("Uninit slot is not initialized")
	}
	return v
}
