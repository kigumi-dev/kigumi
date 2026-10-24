package vm

// mergeSort is rt_core.c's sort_run: stable, taking the right element only when order puts it strictly first.
func (m *Machine) mergeSort(order *obj, items, tmp []*obj) {
	n := len(items)
	if n <= 1 {
		return
	}
	mid := n / 2
	m.mergeSort(order, items[:mid], tmp)
	m.mergeSort(order, items[mid:], tmp)
	l, r, o := 0, mid, 0
	for l < mid && r < n {
		verdict := m.callValueLent(order, []*obj{items[r], items[l]})
		less := m.r.Entity(deref(verdict).ent).Name == "Less"
		m.release(verdict)
		if less {
			tmp[o] = items[r]
			r++
		} else {
			tmp[o] = items[l]
			l++
		}
		o++
	}
	o += copy(tmp[o:], items[l:mid])
	copy(tmp[o:], items[r:])
	copy(items, tmp[:n])
}
