package vm

import "kigumi/internal/sem"

// registerArrayAccel installs Go accelerators for the std array copy bodies the VM runs too slowly; NoAccel runs the bodies.
func (m *Machine) registerArrayAccel() {
	if m.opts.FaultInjectAt > 0 {
		return
	}
	h := m.host
	h["array.Array.tryCopyRange"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		src := deref(a[0])
		n := uint64(len(src.fields))
		lo := min(uint64(deref(a[1]).i), n)
		hi := max(lo, min(uint64(deref(a[2]).i), n))
		if hi-lo >= maxVMAlloc {
			return m.allocError()
		}
		return m.ok(m.sized(mkArray(m.copyElems(src.fields[lo:hi]))))
	}
	h["array.Array.tryReverse"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		src := deref(a[0]).fields
		if uint64(len(src)) >= maxVMAlloc {
			return m.allocError()
		}
		out := make([]*obj, len(src))
		for i, x := range src {
			out[len(src)-1-i] = m.copyElem(x)
		}
		return m.ok(m.sized(mkArray(out)))
	}
	h["array.Array.tryConcat"] = func(m *Machine, _ sem.EntityID, a []*obj) *obj {
		head, tail := deref(a[0]).fields, deref(a[1]).fields
		if uint64(len(head))+uint64(len(tail)) >= maxVMAlloc {
			return m.allocError()
		}
		out := m.copyElems(head)
		for _, x := range tail {
			out = append(out, retain(x))
		}
		return m.ok(m.sized(mkArray(out)))
	}
}

// copyElem is the copy a Kigumi body makes of a Copy element: a fresh one only when a write could reach it.
func (m *Machine) copyElem(x *obj) *obj {
	if m.writableInside(x) {
		return m.copy(x)
	}
	return retain(x)
}

func (m *Machine) copyElems(items []*obj) []*obj {
	out := make([]*obj, len(items), len(items)+4)
	for i, x := range items {
		out[i] = m.copyElem(x)
	}
	return out
}
