package vm

import "kigumi/internal/sem"

// borrowFields mirrors rt_type.borrow_fields: a record never owns what a `&'a T`
// field holds, so reading, overwriting and freeing one all touch no count.
func (m *Machine) borrowFields(ent sem.EntityID) uint64 {
	if mask, ok := m.borrowMask[ent]; ok {
		return mask
	}
	var mask uint64
	if ent != 0 && m.r.Entity(ent).Kind == sem.EntType {
		for i, f := range m.r.TypeDecl(ent).Fields {
			if i < 64 && m.r.Types.Kind(m.r.Entity(f).Type) == sem.KRef {
				mask |= 1 << uint(i)
			}
		}
	}
	m.borrowMask[ent] = mask
	return mask
}

func (m *Machine) borrowField(r *obj, i int) bool {
	return r.k == kRecord && i < 64 && m.borrowFields(r.ent)>>uint(i)&1 != 0
}
