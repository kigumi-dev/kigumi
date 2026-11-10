package vm

import "sync/atomic"

// labelSite is where a label derived for a value lands.
type labelSite struct {
	kind siteKind
	v    *obj
}

type siteKind uint8

const (
	// siteCopy is a fresh copy, so the label reaches no other name.
	siteCopy siteKind = iota
	// siteSelf is the value itself: a copy would lose the writes a container, resource or handle receives.
	siteSelf
)

// siteOf copies a value without identity and a shared immortal aggregate, which no write reaches.
func (m *Machine) siteOf(v *obj) labelSite {
	if n := m.freshScalar(v); n != nil {
		return labelSite{kind: siteCopy, v: n}
	}
	if atomic.LoadInt32(&v.rc) < 0 {
		if c := m.copy(v); c != v {
			return labelSite{kind: siteCopy, v: c}
		}
	}
	return labelSite{kind: siteSelf, v: v}
}

// freshScalar is nil for a kind with identity; unit is copied too, as the shared unit is never freed.
func (m *Machine) freshScalar(v *obj) *obj {
	var n *obj
	switch v.k {
	case kInt:
		n = newObj(kInt)
		n.i, n.w, n.numk = v.i, v.w, v.numk
	case kFloat:
		n = newObj(kFloat)
		n.i, n.numk = v.i, v.numk
	case kBool:
		n = newObj(kBool)
		n.b = v.b
	case kChar:
		n = newObj(kChar)
		n.i = v.i
	case kUnit:
		n = newObj(kUnit)
	case kStr:
		n = newObj(kStr)
		n.setStr(append([]byte(nil), v.str()...))
		m.chargeMemory(n)
	}
	return n
}
