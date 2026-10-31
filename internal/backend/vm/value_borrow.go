package vm

import "kigumi/internal/compiler/sem"

// borrowFields mirrors rt_type.borrow_fields: a record never owns what a `&'a T`
// field holds, so reading, overwriting and freeing one all touch no count.
func (m *Machine) borrowFields(ent sem.EntityID) uint64 {
	m.sh.derived.Lock()
	mask, ok := m.sh.borrowMask[ent]
	m.sh.derived.Unlock()
	if ok {
		return mask
	}
	if ent != 0 && m.r.Entity(ent).Kind == sem.EntType {
		for i, f := range m.r.TypeDecl(ent).Fields {
			if i < 64 && m.r.Types.Kind(m.r.Entity(f).Type) == sem.KRef {
				mask |= 1 << uint(i)
			}
		}
	}
	m.sh.derived.Lock()
	m.sh.borrowMask[ent] = mask
	m.sh.derived.Unlock()
	return mask
}

func (m *Machine) borrowField(r *obj, i int) bool {
	return r.k == kRecord && i < 64 && m.borrowFields(r.ent)>>uint(i)&1 != 0
}
