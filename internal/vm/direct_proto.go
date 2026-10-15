package vm

import "kigumi/internal/hashkey"

const (
	fnvOffset = 14695981039346656037
	fnvPrime  = 1099511628211
)

func (m *Machine) directEq(a, b *obj) bool {
	a, b = unboxed(a), unboxed(b)
	switch a.k {
	case kInt:
		return a.i == b.i
	case kFloat:
		return hashkey.TotalEqualFloat64(a.fl(), b.fl())
	case kBool:
		return a.b == b.b
	case kChar:
		return a.ch() == b.ch()
	case kVariant:
		if a.ent != b.ent {
			return false
		}
	}
	for i := range a.fields {
		if !m.directEq(a.fields[i], b.fields[i]) {
			return false
		}
	}
	return true
}

func (m *Machine) directHash(v *obj) uint64 {
	v = unboxed(v)
	h := uint64(fnvOffset)
	if v.k == kVariant {
		h = uint64(m.variantTag(v)) + 1
		if len(v.fields) == 0 {
			return h
		}
	}
	for _, f := range v.fields {
		h = (h ^ m.leafHash(f)) * fnvPrime
	}
	return h
}

func unboxed(v *obj) *obj {
	if v = deref(v); v.k == kBox {
		return deref(v.inner)
	}
	return v
}

func (m *Machine) leafHash(v *obj) uint64 {
	if v = deref(v); v.k == kRecord || v.k == kVariant || v.k == kArray {
		return m.directHash(v)
	}
	return hashkey.Sum64([]byte(m.hashDisplay(v)))
}

func (m *Machine) variantTag(v *obj) int {
	for i, c := range m.r.TypeDecl(m.r.Entity(v.ent).Parent).Variants {
		if c == v.ent {
			return i
		}
	}
	return 0
}
