package vm

import (
	"kigumi/internal/compiler/sem"
)

// Mirrors rt_call_slot: the receiver is lent unless the method moves it.
func (m *Machine) callMethod(recv *obj, name string, args []*obj, kind sem.RecvKind) *obj {
	var box *obj
	target, t := deref(recv), sem.TypeID(0)
	if target.k == kBox {
		t, box, target = target.dynType(), target, target.inner
	}
	if target.k == kOpaque && target.opaqueOp() == "Error" && name == "message" {
		return mkStr([]byte(target.opaqueData().(string)))
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
	lentCell := box != nil && box.b && kind == sem.RecvMut && len(box.fields) == 1
	// mut self writes through a cell even though the value sits behind the existential.
	switch {
	case lentCell:
		self = box.fields[0]
	case kind == sem.RecvMut:
		cell = mkCell(retain(target))
		self = cell
	}
	r := m.dispatchAt(fn, t, append([]*obj{self}, args...))
	if lentCell {
		box.inner = deref(box.fields[0])
	}
	if kind == sem.RecvMove && box != nil {
		box.inner = nil
		m.release(box)
	}
	if cell != nil {
		if box != nil && !box.b {
			box.inner, cell.inner = cell.inner, nil
			m.release(target)
		}
		m.release(cell)
	}
	return r
}

// dispatchAt runs fn as the method table of a value of dynamic type t would; all leads with the receiver,
// which stays lent.
func (m *Machine) dispatchAt(fn sem.EntityID, t sem.TypeID, all []*obj) *obj {
	_, lowered := m.lowered(fn)
	if e := m.r.Entity(fn); e.Kind == sem.EntFn && e.Flags&sem.EfStd != 0 {
		key := m.stdKey(fn)
		if m.opts.Sandbox && hostEffect(key) {
			m.abort("`" + key + "` is not available at compile time")
		}
		if h, ok := m.hostForEnt(fn, key); ok && m.hostWins(lowered) {
			r := m.callPrim(fn, key, h, all)
			m.releaseArgs(fn, all)
			return r
		}
	}
	if lowered {
		body, all := m.descCall(fn, t, all)
		return m.invoke(body, nil, false, all)
	}
	r := m.callPrim(fn, m.stdKey(fn), nil, all)
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
