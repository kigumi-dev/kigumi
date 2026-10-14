package vm

import (
	"kigumi/internal/sem"
)

// Mirrors rt_call_slot: the receiver is lent unless the method moves it.
func (m *Machine) callMethod(recv *obj, name string, args []*obj, kind sem.RecvKind) *obj {
	var box *obj
	target, t := deref(recv), sem.TypeID(0)
	if target.k == kBox {
		t, box, target = target.dyn, target, target.inner
	}
	if target.k == kOpaque && target.op == "Error" && name == "message" {
		return mkStr([]byte(target.data.(string)))
	}
	if t == 0 {
		t = m.dynType(target)
	}
	set := m.r.MemberSet(t, name)
	if set == 0 {
		m.abort("no method " + name)
	}
	fn := m.r.Overloads[set].Members[0]
	self, cell := target, (*obj)(nil)
	// mut self writes through a cell even though the value sits behind the existential.
	if kind == sem.RecvMut {
		cell = mkCell(retain(target))
		self = cell
	}
	r := m.dispatch(fn, append([]*obj{self}, args...))
	if kind == sem.RecvMove && box != nil {
		box.inner = nil
		m.release(box)
	}
	if cell != nil {
		if box != nil {
			box.inner, cell.inner = cell.inner, nil
			m.release(target)
		}
		m.release(cell)
	}
	return r
}

// all leads with the receiver, which stays lent.
func (m *Machine) dispatch(fn sem.EntityID, all []*obj) *obj {
	f, lowered := m.p.ByEnt[fn]
	if e := m.r.Entity(fn); e.Kind == sem.EntFn && e.Flags&sem.EfStd != 0 {
		key := m.stdKey(fn)
		if m.opts.Sandbox && hostEffect(key) {
			m.abort("`" + key + "` is not available at compile time")
		}
		if h, ok := m.hostForEnt(fn, key); ok && m.hostWins(lowered) {
			r := h(m, fn, all)
			m.chargeMemory(r)
			m.releaseArgs(fn, all)
			return r
		}
	}
	if lowered {
		return m.invoke(f, nil, false, all)
	}
	r := m.std(m.stdKey(fn), all)
	m.chargeMemory(r)
	m.releaseArgs(fn, all)
	return r
}

func (m *Machine) releaseArgs(fn sem.EntityID, all []*obj) {
	for i := 1; i < len(all); i++ {
		if !m.p.ParamBorrowed(fn, i) {
			m.release(all[i])
		}
	}
}
