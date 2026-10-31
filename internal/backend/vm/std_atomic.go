package vm

import (
	"strings"

	"kigumi/internal/compiler/sem"
)

// One lock keeps every atomic sequentially consistent whatever order it names, and ordered against plain static reads.
func (m *Machine) atomicApply(op string, get func() *obj, set func(*obj), args []*obj) *obj {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	old := get()
	if op == "store" {
		set(scalarCopy(deref(args[0])))
		return unitObj
	}
	res := scalarCopy(old)
	switch op {
	case "load":
	case "exchange":
		set(scalarCopy(deref(args[0])))
	case "compareExchange":
		if scalarEqual(old, deref(args[0])) {
			set(scalarCopy(deref(args[1])))
		}
	default:
		set(mkInt(atomicInt(op, old.i, deref(args[0]).i, old.nkind()), old.nkind()))
	}
	return res
}

func atomicInt(op string, old, v int64, nk int) int64 {
	switch op {
	case "add":
		old += v
	case "sub":
		old -= v
	case "and":
		old &= v
	case "or":
		old |= v
	case "xor":
		old ^= v
	}
	return truncateTo(old, nk)
}

func scalarCopy(v *obj) *obj {
	switch v.k {
	case kInt:
		return mkInt(v.i, v.nkind())
	case kBool:
		return mkBool(v.b)
	}
	return mkOpaque("Ptr", v.opaqueData())
}

func scalarEqual(a, b *obj) bool {
	switch a.k {
	case kInt:
		return a.i == b.i
	case kBool:
		return a.b == b.b
	}
	return a.opaqueData() == b.opaqueData()
}

func (m *Machine) atomicStatic(ent sem.EntityID, op string, args []*obj) *obj {
	m.loadGlobal(ent)
	get := func() *obj { return m.sh.globals[ent] }
	set := func(v *obj) {
		m.immortalizeDeep(v)
		m.sh.globals[ent] = v
	}
	return m.atomicApply(strings.TrimPrefix(op, "atomic."), get, set, args)
}

func (m *Machine) atomicCell(key string, cell *obj, args []*obj) *obj {
	c := cell.opaqueData().(*obj)
	get := func() *obj { return c.inner }
	set := func(v *obj) {
		old := c.inner
		c.inner = v
		m.release(old)
	}
	return m.atomicApply(strings.ToLower(key[len("mem.UnsafeCell.atomic"):][:1])+key[len("mem.UnsafeCell.atomic")+1:], get, set, args)
}
