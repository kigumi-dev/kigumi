package vm

// index is indexCore with the SecretSan checks: a secret index is a sink, and
// an element of a shadowed aggregate carries the aggregate's label.
func (m *Machine) index(base, idx *obj) *obj {
	m.traceIndexAt("index", base, idx)
	if !m.opts.SecretSan {
		return m.indexCore(base, idx)
	}
	m.checkIndex(idx)
	return m.deriveTaint(m.indexCore(base, idx), m.taintOf(base))
}

func (m *Machine) indexCore(base, idx *obj) *obj {
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
				m.abort("slice out of range")
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

// setIndex is setIndexCore with the SecretSan index check and a recomputed
// label for the container after the store.
func (m *Machine) setIndex(base, idx, v *obj) *obj {
	m.traceIndexAt("setindex", base, idx)
	if !m.opts.SecretSan {
		return m.setIndexCore(base, idx, v)
	}
	m.checkIndex(idx)
	old := m.setIndexCore(base, idx, v)
	m.relabelContainer(base)
	return old
}

// relabelContainer recomputes an aggregate's coarse label from its elements,
// so overwriting a secret element with a public one stops shadowing the whole.
func (m *Machine) relabelContainer(v *obj) {
	v = derefRaw(v)
	if v == nil {
		return
	}
	switch v.k {
	case kArray:
		var t shadow
		for _, e := range v.fields {
			t = t.merge(m.taintOf(e))
		}
		m.replaceLabel(v, t)
	case kRecord, kVariant:
		var t shadow
		for _, e := range v.fields {
			t = t.merge(m.taintOf(e))
		}
		m.replaceLabel(v, t)
	}
}

func (m *Machine) setIndexCore(base, idx, v *obj) *obj {
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
