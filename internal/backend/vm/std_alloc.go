package vm

import "kigumi/internal/compiler/sem"

// Shared handles are freely copied, so borrowing is checked dynamically:
// borrow is 0 when free, -1 for an exclusive borrow, or the shared borrow count.
type sharedCtrl struct {
	cell   *obj
	shared *obj
	weak   int
	alive  bool
	borrow int
}

// arenaState models an arena in the VM: Go owns every allocation, so only
// explicit Blocks are attributed to it, which is what reset/close count.
type arenaState struct {
	live   int64
	bytes  int64
	closed bool
}

// blockState is one Block: Go-owned zeroed bytes plus the arena they count
// against and return to when the Block drops.
type blockState struct {
	arena *arenaState
	data  []byte
}

// currentArena is the innermost `allocator` block's arena, or the root arena
// the VM makes for allocation outside one.
func (m *Machine) currentArena() *arenaState {
	if n := len(m.allocStack); n > 0 {
		return m.allocStack[n-1]
	}
	if m.rootArena == nil {
		m.rootArena = &arenaState{}
	}
	return m.rootArena
}

func (m *Machine) busyError(outstanding int64) *obj {
	busy := mkRecord(m.lookup("std/alloc", "Busy"), []*obj{mkInt(outstanding, m.numKind(sem.TyUsize))})
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{busy})
}

// alloc implements std/alloc: Shared / Weak by a control block, arenas as
// handles over arenaState (the VM allocates from Go, so allocated() reports 0).
func (m *Machine) alloc(key string, a []*obj, a0, a1 *obj) *obj {
	switch key {
	case "alloc.Arena.create":
		if m.faultTick() {
			return m.errMsg("out of memory")
		}
		return m.ok(mkOpaque("AllocatorHandle", &arenaState{}))
	case "alloc.Arena.tryCreate":
		if m.faultTick() {
			return m.allocError()
		}
		return m.ok(mkOpaque("AllocatorHandle", &arenaState{}))
	case "alloc.AllocatorHandle.scope":
		if a0.opaqueData().(*arenaState).closed {
			m.abort("arena is closed")
		}
		// rt_std_dispatch.inc OP_ALLOCATOR_SCOPE builds the handle with
		// opaque_in, one fatal allocation before the retained receiver.
		m.allocHeader()
		return mkOpaque("AllocatorScope", retain(a0))
	case "alloc.AllocatorHandle.allocated":
		return mkInt(0, 64|256)
	case "alloc.AllocatorHandle.reset":
		st := a0.opaqueData().(*arenaState)
		if st.closed {
			m.abort("arena is closed")
		}
		if st.live > 0 {
			return m.busyError(st.live)
		}
		st.bytes = 0
		return m.ok(unitObj)
	case "alloc.AllocatorHandle.close":
		st := a0.opaqueData().(*arenaState)
		if st.live > 0 {
			return m.busyError(st.live)
		}
		st.closed, st.bytes = true, 0
		return m.ok(unitObj)
	case "alloc.Block.tryNew":
		n := a0.i
		if n < 0 || uint64(n) >= maxVMAlloc {
			return m.allocError()
		}
		st := m.currentArena()
		if st.closed {
			m.abort("arena is closed")
		}
		if m.planRefused(key) {
			return m.allocError()
		}
		b := &blockState{arena: st, data: make([]byte, n)}
		st.live++
		st.bytes += n
		return m.okPlanned(key, mkOpaque("Block", b))
	case "alloc.Block.len":
		b := a0.opaqueData().(*blockState)
		return mkInt(int64(len(b.data)), m.numKind(sem.TyUsize))
	case "alloc.Block.get":
		b := a0.opaqueData().(*blockState)
		if a1.i < 0 || a1.i >= int64(len(b.data)) {
			m.abortIndex(a1.i, int64(len(b.data)))
		}
		return mkInt(int64(b.data[a1.i]), 8)
	case "alloc.Block.set":
		b := a0.opaqueData().(*blockState)
		value := deref(a[2])
		if a1.i < 0 || a1.i >= int64(len(b.data)) {
			m.abortIndex(a1.i, int64(len(b.data)))
		}
		b.data[a1.i] = byte(value.i)
		return unitObj
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
		return m.copiedResult(key, a0.opaqueData().(*obj).inner)
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

func (m *Machine) popArena() {
	if n := len(m.allocStack); n > 0 {
		m.allocStack = m.allocStack[:n-1]
	}
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
	case "AllocGuard":
		// The guard marks the dynamic extent of one allocator block; its
		// final release is where that extent ends.
		m.popArena()
		scope, _ := v.opaqueData().(*obj)
		return scope
	}
	return nil
}

func (m *Machine) freeOpaqueAt(v *obj, d int) {
	switch v.opaqueOp() {
	case "Shared", "AllocatorScope", "UnsafeCell", "Box", "AllocGuard":
		m.releaseAt(m.opaqueOwned(v), d)
	case "Block":
		b := v.opaqueData().(*blockState)
		b.arena.live--
		b.data = nil
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
