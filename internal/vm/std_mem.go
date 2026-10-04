package vm

// memStd implements std/mem: UnsafeCell is an opaque handle around a plain
// VM cell, so the general deref pass does not unwrap it by accident.
func (m *Machine) memStd(key string, a []*obj, a0, a1 *obj) *obj {
	switch key {
	case "mem.UnsafeCell.new":
		return mkOpaque("UnsafeCell", mkCell(retain(a0)))
	case "mem.UnsafeCell.get":
		return m.copy(a0.data.(*obj).inner)
	case "mem.UnsafeCell.set":
		cell := a0.data.(*obj)
		old := cell.inner
		cell.inner = retain(a1)
		m.release(old)
		return unitObj
	}
	return nil
}
