package vm

import (
	"kigumi/internal/compiler/sem"
)

// simdFn runs the std/simd primitives the native backend lowers to vector
// instructions. The lane values are the same boxed scalars the rest of the
// VM uses, so the two backends agree bit for bit; nil is another key.
func (m *Machine) simdFn(key string, a []*obj, a0, a1 *obj) *obj {
	switch key {
	case "simd.Vec.splat":
		_, n, ent, ok := m.simdResultVec()
		if !ok {
			panic(&Unsupported{What: "an indirect `Vec.splat` call"})
		}
		items := make([]*obj, n)
		for i := range items {
			items[i] = m.simdCopyLane(a0)
		}
		return mkRecord(ent, []*obj{mkArray(items)})
	case "simd.Vec.fromArray":
		lanes := simdLanesOf(a0)
		if lanes == nil {
			m.abort("simd: fromArray needs a FixedArray")
		}
		items := make([]*obj, len(lanes))
		for i, v := range lanes {
			items[i] = m.simdCopyLane(v)
		}
		return mkRecord(m.simdVecEnt(), []*obj{mkArray(items)})
	case "simd.Vec.toArray":
		lanes := simdLanesOf(a0)
		if lanes == nil {
			m.abort("simd: toArray needs a Vec")
		}
		items := make([]*obj, len(lanes))
		for i, v := range lanes {
			items[i] = m.simdCopyLane(v)
		}
		return mkArray(items)
	case "simd.Vec.add", "simd.Vec.sub", "simd.Vec.mul":
		l, r := simdLanesOf(a0), simdLanesOf(a1)
		if l == nil || r == nil || len(l) != len(r) {
			m.abort("simd: lane counts differ")
		}
		op := key[len("simd.Vec."):]
		items := make([]*obj, len(l))
		for i := range items {
			items[i] = m.simdBinLane(op, l[i], r[i])
		}
		return mkRecord(m.simdVecEnt(), []*obj{mkArray(items)})
	}
	return nil
}

// simdResultVec reads the result type of the call being dispatched; the plan
// only lowers a direct call, so the instantiated Vec is always there.
func (m *Machine) simdResultVec() (elem sem.TypeID, n int64, ent sem.EntityID, ok bool) {
	elem, n, ok = m.r.SimdVecShape(m.callRet)
	if !ok {
		return 0, 0, 0, false
	}
	ent = m.r.Types.Node(m.callRet).Ent
	if ent == 0 {
		ent = m.simdVecEnt()
	}
	return elem, n, ent, ent != 0
}

// simdVecEnt is the Vec type declaration, shared by every instantiation.
func (m *Machine) simdVecEnt() sem.EntityID { return m.lookup("std/simd", "Vec") }

// simdLanesOf reads a Vec's lanes or a FixedArray's elements.
func simdLanesOf(v *obj) []*obj {
	v = deref(v)
	if v == nil {
		return nil
	}
	if v.k == kRecord {
		if len(v.fields) != 1 {
			return nil
		}
		v = deref(v.fields[0])
		if v == nil {
			return nil
		}
	}
	switch v.k {
	case kArray:
		return v.fields
	case kRawArray:
		r := v.rawArr()
		out := make([]*obj, len(r.raw))
		for i := range out {
			out[i] = r.at(int64(i))
		}
		return out
	}
	return nil
}

// simdCopyLane makes a lane value the result owns; lanes are scalars, so a
// fresh box is enough.
func (m *Machine) simdCopyLane(v *obj) *obj {
	v = deref(v)
	switch v.k {
	case kFloat:
		return m.newFloat(v.fl(), v.nkind())
	case kInt:
		return m.newInt(v.i, v.nkind())
	}
	m.abort("simd: a lane is not a number")
	return nil
}

// simdBinLane applies one lane operation, wrapping at the lane's width.
func (m *Machine) simdBinLane(op string, a, b *obj) *obj {
	a, b = deref(a), deref(b)
	if a.k == kFloat {
		x, y := a.fl(), b.fl()
		switch op {
		case "add":
			return m.newFloat(x+y, a.nkind())
		case "sub":
			return m.newFloat(x-y, a.nkind())
		default:
			return m.newFloat(x*y, a.nkind())
		}
	}
	if a.k != kInt || b.k != kInt {
		m.abort("simd: a lane is not a number")
	}
	x, y, nk := a.i, b.i, a.nkind()
	switch op {
	case "add":
		return m.newInt(truncateTo(x+y, nk), nk)
	case "sub":
		return m.newInt(truncateTo(x-y, nk), nk)
	default:
		return m.newInt(truncateTo(x*y, nk), nk)
	}
}
