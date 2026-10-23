package vm

// memStd implements std/mem: UnsafeCell is an opaque handle around a plain
// VM cell, so the general deref pass does not unwrap it by accident.
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
