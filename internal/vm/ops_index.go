package vm

func (m *Machine) index(base, idx *obj) *obj {
	base, idx = derefRaw(base), deref(idx)
	if base.k == kRawArray {
		if idx.k == kRecord {
			base = deref(base)
		} else {
			r := base.rawArr()
			if idx.i < 0 || idx.i >= int64(len(r.raw)) {
				m.abortIndex(idx.i, int64(len(r.raw)))
			}
			return r.at(idx.i)
		}
	}
	if idx.k == kRecord {
		lo, hi := deref(idx.fields[0]).i, deref(idx.fields[1]).i
		if deref(idx.fields[2]).b {
			hi++
		}
		if base.k == kStr || base.k == kBytes {
			if lo < 0 || hi > int64(len(base.str())) || lo > hi {
				m.abort("slice is not on character boundaries")
			}
			if base.k == kStr && (!boundary(base.str(), lo) || !boundary(base.str(), hi)) {
				m.abort("slice is not on character boundaries")
			}
			out := newObj(base.k)
			out.setStr(append([]byte{}, base.str()[lo:hi]...))
			m.chargeMemory(out)
			return out
		}
		if lo < 0 || hi > int64(len(base.fields)) || lo > hi {
			m.abort("slice out of range")
		}
		out := mkArray(base.fields[lo:hi])
		for _, x := range out.fields {
			retain(x)
		}
		m.chargeMemory(out)
		return out
	}
	i := idx.i
	if base.k == kStr || base.k == kBytes {
		if i < 0 || i >= int64(len(base.str())) {
			m.abortIndex(i, int64(len(base.str())))
		}
		return mkInt(int64(base.str()[i]), 8)
	}
	if base.k != kArray || i < 0 || i >= int64(len(base.fields)) {
		m.abortIndex(i, int64(len(base.fields)))
	}
	return retain(base.fields[i])
}

func (m *Machine) setIndex(base, idx, v *obj) *obj {
	base = derefRaw(base)
	i := deref(idx).i
	if base.k == kRawArray {
		r := base.rawArr()
		if i < 0 || i >= int64(len(r.raw)) {
			m.abortIndex(i, int64(len(r.raw)))
		}
		old := r.at(i)
		if r.store(i, v) {
			m.release(v)
			return old
		}
		base.expand()
		prev := base.fields[i]
		base.fields[i] = v
		m.release(old)
		return prev
	}
	if base.k != kArray || i < 0 || i >= int64(len(base.fields)) {
		m.abortIndex(i, int64(len(base.fields)))
	}
	old := base.fields[i]
	base.fields[i] = v
	return old
}
