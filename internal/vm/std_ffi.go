package vm

import "kigumi/internal/sem"

// The VM has no C ABI: CValue and Pin contents stay VM values in a table, and a pointer to one is just the opaque handle (null is the only other pointer).
// Foreign memory support stops with a pointer to `kigumi build`, gated by Supports.
func (m *Machine) registerFFI() {
	h := m.host
	handleOf := func(v *obj) int64 { return deref(deref(v).fields[0]).i }
	ptrHandle := func(m *Machine, v *obj) int64 {
		hd, ok := deref(v).opaqueData().(int64)
		if !ok {
			m.abort("this pointer does not address a CValue; run the program with `kigumi build`")
		}
		return hd
	}
	// A generic return type (`Pin[T]`) carries no entity, so the handle
	// records are looked up by name.
	newHandle := func(m *Machine, name string, v *obj) *obj {
		return mkRecord(m.lookup("std/ffi", name), []*obj{mkInt(m.addCValue(v), m.numKind(sem.TyI64))})
	}
	h["ffi.null"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Ptr", nil) }
	h["ffi.isNull"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkBool(deref(a[0]).opaqueData() == nil) }
	for _, k := range []string{"ffi.toConst", "ffi.toMut", "ffi.cast"} {
		h[k] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return retain(deref(a[0])) }
	}
	h["ffi.sizeOf"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		ref := m.siteFn.Locals[m.site.Args[0]].Type
		size, _ := m.cLayout(m.r.Types.Node(ref).Elem)
		return mkInt(size, m.numKind(sem.TyUsize))
	}
	h["ffi.alignOf"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		ref := m.siteFn.Locals[m.site.Args[0]].Type
		_, align := m.cLayout(m.r.Types.Node(ref).Elem)
		return mkInt(align, m.numKind(sem.TyUsize))
	}
	h["ffi.CValue.new"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return newHandle(m, "CValue", m.copy(deref(a[0]))) }
	h["ffi.Pin.new"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return newHandle(m, "Pin", retain(deref(a[0]))) }
	for _, k := range []string{"ffi.CValue.ptr", "ffi.Pin.ptr"} {
		h[k] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Ptr", handleOf(a[0])) }
	}
	// self is a runtime-primitive arg (inPlace), so the caller also runs
	// Pin.drop on self right after this call; zeroing self's own handle
	// field first makes that re-entry a no-op (see Pin.drop) without
	// touching the slot, so a pointer already handed out via .ptr() keeps
	// addressing the same live value.
	h["ffi.Pin.release"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		hd := handleOf(a[0])
		rewriteInt(deref(a[0]), 0, 0)
		return mkOpaque("Ptr", hd)
	}
	h["ffi.CValue.get"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return m.copy(m.cvalue(handleOf(a[0]))) }
	h["ffi.CValue.set"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		m.replaceCValue(handleOf(a[0]), m.copy(deref(a[1])))
		return unitObj
	}
	h["ffi.CValue.drop"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		m.replaceCValue(handleOf(a[0]), nil)
		return unitObj
	}
	h["ffi.readRecord"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return m.copy(m.cvalue(ptrHandle(m, a[0]))) }
	h["ffi.writeRecord"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		m.replaceCValue(ptrHandle(m, a[0]), m.copy(deref(a[1])))
		return unitObj
	}
	h["ffi.Pin.with"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		hd, ok := deref(a[0]).opaqueData().(int64)
		if !ok {
			m.abort("this pointer does not address a Pin; run the program with `kigumi build`")
		}
		v := m.cvalue(hd)
		if v == nil {
			m.abort("this pointer addresses a Pin that was already dropped")
		}
		// f borrows the value, so the reference invoke takes is the only one.
		return m.callValue(a[1], []*obj{v})
	}
	// Call-scoped: the handle lives only for f's call, unlike Pin's handle,
	// which lives until an explicit drop.
	withAddress := func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		hd := m.addCValue(retain(deref(a[0])))
		r := m.callValue(a[1], []*obj{mkOpaque("Ptr", hd)})
		m.replaceCValue(hd, nil)
		return r
	}
	h["ffi.withAddress"] = withAddress
	h["ffi.withAddressConst"] = withAddress
	h["ffi.Pin.drop"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		// Zero from Pin.release means this is the caller's bogus re-drop
		// (see above), not a real one: skip it.
		if hd := handleOf(a[0]); hd != 0 {
			m.replaceCValue(hd, nil)
		}
		return unitObj
	}
	h["ffi.Pin.reclaim"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		hd, ok := deref(a[0]).opaqueData().(int64)
		if !ok {
			m.abort("this pointer does not address a Pin; run the program with `kigumi build`")
		}
		if m.cvalue(hd) == nil {
			m.abort("this pointer addresses a Pin that was already dropped")
		}
		return mkRecord(m.lookup("std/ffi", "Pin"), []*obj{mkInt(hd, m.numKind(sem.TyI64))})
	}
}

// retType is the callee's declared return type.
func (m *Machine) retType(fn sem.EntityID) sem.TypeID {
	return m.r.Types.Node(m.r.Fn(fn).Sig).Elem
}

// cLayout is the C size and alignment of a layout(C) record as the native
// runtime computes it: natural alignment unless packed, widened by an
// explicit align.
func (m *Machine) cLayout(t sem.TypeID) (size, align int64) {
	tt := m.r.Types
	switch {
	case tt.IsNumeric(t):
		b := int64(tt.Width(t) / 8)
		return b, b
	case t == sem.TyBool:
		return 1, 1
	case t == sem.TyChar:
		return 4, 4
	case tt.Kind(t) == sem.KPtr || tt.IsCFn(t):
		p := int64(tt.PtrBits() / 8)
		return p, p
	}
	if u := m.underlyingRepr(t); u != t {
		return m.cLayout(u)
	}
	info := m.r.TypeDecl(tt.Node(t).Ent)
	packed := info.Layout == "packed"
	var off, maxAlign int64 = 0, 1
	for _, f := range info.Fields {
		fs, fa := m.cLayout(m.r.Entity(f).Type)
		if packed {
			fa = 1
		}
		off = (off+fa-1)/fa*fa + fs
		maxAlign = max(maxAlign, fa)
	}
	maxAlign = max(maxAlign, info.Align)
	return (off + maxAlign - 1) / maxAlign * maxAlign, maxAlign
}
