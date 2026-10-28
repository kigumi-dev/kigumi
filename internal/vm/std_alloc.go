package vm

// sharedCtrl is the control block behind Shared and Weak: the cell holding
// the value, whether a strong reference still exists, and the weak count.
// borrow is a RefCell-style dynamic borrow flag (0 free, -1 exclusive, >0
// shared count) since Shared's receiver is a freely-copyable handle the
// static borrow checker does not track.
type sharedCtrl struct {
	cell   *obj
	shared *obj
	weak   int
	alive  bool
	borrow int
}

// alloc implements std/alloc: Shared / Weak by a control block, arenas as
// plain handles (the VM allocates from Go, so allocated() reports 0).
func (m *Machine) alloc(key string, a []*obj, a0, a1 *obj) *obj {
	switch key {
	case "alloc.Arena.create":
		if m.faultTick() {
			return m.errMsg("out of memory")
		}
		return m.ok(mkOpaque("AllocatorHandle", nil))
	case "alloc.Arena.tryCreate":
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(mkOpaque("AllocatorHandle", nil))
	case "alloc.AllocatorHandle.scope":
		return mkOpaque("AllocatorScope", retain(a0))
	case "alloc.AllocatorHandle.allocated":
		return mkInt(0, 64|256)
	case "alloc.Shared.tryNew":
		if m.planRefused(key) {
			return m.allocError()
		}
		c := &sharedCtrl{cell: mkCell(retain(a0)), alive: true}
		c.shared = mkOpaque("Shared", c)
		return m.okPlanned(key, c.shared)
	case "alloc.Shared.clone", "alloc.Weak.clone":
		return retain(a0)
	case "alloc.Shared.set":
		c := a0.opaqueData().(*sharedCtrl)
		if c.borrow != 0 {
			m.abort("Shared is already borrowed")
		}
		cell := c.cell
		old := cell.inner
		cell.inner = retain(a1)
		m.release(old)
		return unitObj
	case "alloc.Shared.with":
		c := a0.opaqueData().(*sharedCtrl)
		if c.borrow < 0 {
			m.abort("Shared is exclusively borrowed")
		}
		c.borrow++
		defer func() { c.borrow-- }()
		return m.callValueLent(a1, []*obj{c.cell.inner})
	case "alloc.Shared.withMut":
		c := a0.opaqueData().(*sharedCtrl)
		if c.borrow != 0 {
			m.abort("Shared is already borrowed")
		}
		c.borrow = -1
		defer func() { c.borrow = 0 }()
		// The callback writes through c.cell itself, same as any
		// other &mut Cell store, instead of a snapshot of its content.
		return m.callValueLent(a1, []*obj{c.cell})
	case "alloc.Box.tryNew":
		if m.planRefused(key) {
			return m.allocError()
		}
		return m.okPlanned(key, mkOpaque("Box", mkCell(retain(a0))))
	case "alloc.Box.get":
		return m.copy(a0.opaqueData().(*obj).inner)
	case "alloc.Box.set":
		cell := a0.opaqueData().(*obj)
		old := cell.inner
		cell.inner = retain(a1)
		m.release(old)
		return unitObj
	case "alloc.Box.with":
		return m.callValueLent(a1, []*obj{a0.opaqueData().(*obj).inner})
	case "alloc.Box.withMut":
		return m.callValueLent(a1, []*obj{a0.opaqueData().(*obj)})
	case "alloc.Box.into":
		return retain(a0.opaqueData().(*obj).inner)
	case "alloc.Weak.upgrade":
		c := a0.opaqueData().(*sharedCtrl)
		if c.alive {
			return m.some(retain(c.shared))
		}
		return m.none()
	}
	return nil
}

func (m *Machine) opaqueOwned(v *obj) *obj {
	switch v.opaqueOp() {
	case "Shared":
		c := v.opaqueData().(*sharedCtrl)
		c.alive = false
		return c.cell
	case "AllocatorScope", "UnsafeCell", "Box":
		h, _ := v.opaqueData().(*obj)
		return h
	}
	return nil
}

func (m *Machine) freeOpaqueAt(v *obj, d int) {
	switch v.opaqueOp() {
	case "Shared", "AllocatorScope", "UnsafeCell", "Box":
		m.releaseAt(m.opaqueOwned(v), d)
	case "Uninit":
		cell := v.opaqueData().(*obj)
		cell.inner = nil
		m.releaseAt(cell, d)
	case "Weak":
		c := v.opaqueData().(*sharedCtrl)
		c.weak--
	case "Future":
		m.freeFuture(v.opaqueData().(*future))
	case "Executor":
		m.closeExecutor(v.opaqueData().(*executor))
	case "Task":
		m.detachTask(v.opaqueData().(*task))
	}
}
