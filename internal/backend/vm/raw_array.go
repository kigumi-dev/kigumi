package vm

import (
	"slices"
	"sync"
	"unsafe"
)

type rawArrObj struct {
	obj
	raw  []int64
	elem kind
	nk   int
}

var compactMin = 64

var expandMu sync.Mutex

func (v *obj) rawArr() *rawArrObj { return (*rawArrObj)(unsafe.Pointer(v)) }

func (r *rawArrObj) at(i int64) *obj {
	w := r.raw[i]
	switch r.elem {
	case kInt:
		return mkInt(w, r.nk)
	case kFloat:
		v := newObj(kFloat)
		v.i = w
		v.setNk(r.nk)
		return v
	case kChar:
		return mkChar(rune(w))
	}
	return mkBool(w != 0)
}

// borrowed is at for a reference nothing releases: the object is never freed.
func (r *rawArrObj) borrowed(i int64) *obj {
	v := r.at(i)
	if v.rc > 0 {
		setImmortal(v)
	}
	return v
}

func (r *rawArrObj) fits(v *obj) bool {
	if v == nil || v.k != r.elem {
		return false
	}
	return r.elem == kBool || r.elem == kChar || v.nkind() == r.nk
}

func (r *rawArrObj) store(i int64, v *obj) bool {
	v = deref(v)
	if !r.fits(v) {
		return false
	}
	if r.elem == kBool {
		r.raw[i] = 0
		if v.b {
			r.raw[i] = 1
		}
		return true
	}
	r.raw[i] = v.i
	return true
}

// expand keeps raw so a lane that already saw the compact kind can finish reading it.
func (v *obj) expand() {
	expandMu.Lock()
	defer expandMu.Unlock()
	if v.k != kRawArray {
		return
	}
	r := v.rawArr()
	items := make([]*obj, len(r.raw))
	for i := range items {
		items[i] = r.at(int64(i))
	}
	v.fields = items
	v.k = kArray
}

func compactElem(a *obj) (elem kind, nk int, ok bool) {
	first := deref(a.fields[0])
	if first.k != kInt && first.k != kFloat && first.k != kBool && first.k != kChar {
		return 0, 0, false
	}
	if first.k == kInt && is128(first.nkind()) {
		return 0, 0, false
	}
	elem, nk = first.k, first.nkind()
	for _, e := range a.fields {
		if e.k != elem || (elem != kBool && elem != kChar && e.nkind() != nk) {
			return 0, 0, false
		}
	}
	return elem, nk, true
}

func (m *Machine) compact(a *obj) *obj {
	if a.k != kArray || len(a.fields) < compactMin || len(a.fields) == 0 {
		return retain(a)
	}
	elem, nk, ok := compactElem(a)
	if !ok {
		return retain(a)
	}
	out := newObj(kRawArray)
	r := out.rawArr()
	r.elem, r.nk = elem, nk
	r.raw = make([]int64, len(a.fields))
	for i, e := range a.fields {
		if elem == kBool {
			if e.b {
				r.raw[i] = 1
			}
			continue
		}
		r.raw[i] = e.i
	}
	m.chargeMemory(out)
	return out
}

func (m *Machine) copyRaw(v *obj) *obj {
	out := newObj(kRawArray)
	r, src := out.rawArr(), v.rawArr()
	r.elem, r.nk, r.raw = src.elem, src.nk, slices.Clone(src.raw)
	m.chargeMemory(out)
	return m.inheritLabel(v, out)
}

func (m *Machine) rawWithMut(a []*obj) *obj {
	base := derefRaw(a[0])
	if base.k != kRawArray {
		return nil
	}
	r, i := base.rawArr(), deref(a[1]).i
	if i < 0 || i >= int64(len(r.raw)) {
		return m.none()
	}
	cell := mkCell(r.at(i))
	defer func() {
		inner := cell.inner
		cell.inner = nil
		if base.k == kRawArray && r.store(i, inner) {
			m.release(inner)
			m.release(cell)
			return
		}
		base.expand()
		base.fields[i] = inner
		m.release(cell)
	}()
	return m.some(m.callValueLent(a[2], []*obj{cell}))
}
