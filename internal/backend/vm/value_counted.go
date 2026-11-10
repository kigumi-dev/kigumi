package vm

import "kigumi/internal/compiler/sem"

// countedPayloads names, for a counted Option or Result variant, the payload types; nil for any other value.
func (m *Machine) countedPayloads(t sem.TypeID, v *obj) []sem.TypeID {
	if v == nil || v.k != kVariant || !m.r.CountedAdt(t) {
		return nil
	}
	pts := m.r.AggPayload(t, v.ent)
	if len(pts) != len(v.fields) {
		return nil
	}
	return pts
}

// crossCopy copies the shell of a counted value and retains its references, the way native's frame copy shares them.
func (m *Machine) crossCopy(t sem.TypeID, v *obj) *obj {
	d := deref(v)
	pts := m.countedPayloads(t, d)
	if pts == nil {
		return m.copy(v)
	}
	c := mkVariant(d.ent, d.fields)
	for i, pt := range pts {
		switch {
		case m.r.CountedSlot(pt):
			c.fields[i] = retain(c.fields[i])
		case m.r.CountedAdt(pt):
			c.fields[i] = m.crossCopy(pt, c.fields[i])
		default:
			c.fields[i] = m.copy(c.fields[i])
		}
	}
	m.chargeMemory(c)
	if m.opts.SecretSan {
		m.mergeLabel(c, m.chainLabel(v))
	}
	return c
}

// crossTicks counts native's allocations boxing v: a counted value's references go into the box as they are.
func (m *Machine) crossTicks(t sem.TypeID, v *obj) int {
	d := deref(v)
	pts := m.countedPayloads(t, d)
	if pts == nil {
		return boxTicks(v)
	}
	n := 2
	for i, pt := range pts {
		switch {
		case m.r.CountedSlot(pt):
		case m.r.CountedAdt(pt):
			n += m.crossTicks(pt, d.fields[i])
		default:
			n += boxTicks(d.fields[i])
		}
	}
	return n
}
