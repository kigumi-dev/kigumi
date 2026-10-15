package vm

import "sync/atomic"

const (
	internMin = -128
	internN   = 1152
)

var internTables [8]atomic.Pointer[[internN]obj]

func internSlot(nk int) int {
	if nk&^(255|256) != 0 {
		return -1
	}
	var s int
	switch nk & 255 {
	case 8:
		s = 0
	case 16:
		s = 1
	case 32:
		s = 2
	case 64:
		s = 3
	default:
		return -1
	}
	if nk&256 != 0 {
		s += 4
	}
	return s
}

// internInt is the shared immortal for a small integer of kind nk, nil outside the table.
func internInt(i int64, nk int) *obj {
	s := internSlot(nk)
	if s < 0 {
		return nil
	}
	t := internTables[s].Load()
	if t == nil {
		t = new([internN]obj)
		for n := range t {
			t[n] = obj{k: kInt, rc: -1, i: int64(n) + internMin, nk: nk}
		}
		if !internTables[s].CompareAndSwap(nil, t) {
			t = internTables[s].Load()
		}
	}
	return &t[i-internMin]
}

// rewriteInt sets an Int field, replacing the slot's object when it is a shared immortal.
func rewriteInt(r *obj, idx int, v int64) {
	slot := &r.fields[idx]
	for (*slot).k == kCell {
		slot = &(*slot).inner
	}
	f := *slot
	if atomic.LoadInt32(&f.rc) < 0 {
		*slot = mkInt(v, f.nk)
		return
	}
	f.i = v
}

const scalarPoolMax = 512

// newInt is mkInt that reuses an object this lane freed instead of allocating.
func (m *Machine) newInt(i int64, nk int) *obj {
	if nk == 0 {
		nk = 64 | 256
	}
	if uint64(i-internMin) < internN {
		if v := internInt(i, nk); v != nil {
			return v
		}
	}
	if n := len(m.pool); n > 0 {
		v := m.pool[n-1]
		m.pool = m.pool[:n-1]
		if liveTrack.Load() {
			liveMade.Add(1)
		}
		v.k, v.rc, v.i, v.nk = kInt, 1, i, nk
		return v
	}
	return mkInt(i, nk)
}

func (m *Machine) newFloat(f float64, nk int) *obj {
	if n := len(m.pool); n > 0 {
		v := m.pool[n-1]
		m.pool = m.pool[:n-1]
		if liveTrack.Load() {
			liveMade.Add(1)
		}
		if nk&255 == 32 {
			f = float64(float32(f))
		}
		v.k, v.rc, v.f, v.nk = kFloat, 1, f, nk
		return v
	}
	return mkFloat(f, nk)
}
