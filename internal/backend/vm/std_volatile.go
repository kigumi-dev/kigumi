package vm

import "kigumi/internal/compiler/sem"

// The VM has no device memory: a volatile access through a pointer reads and writes the CValue it addresses.
func (m *Machine) registerMem() {
	h := m.host
	handle := func(m *Machine, what string, p *obj) int64 {
		hd, ok := deref(p).opaqueData().(int64)
		if !ok {
			m.abort("this pointer does not address a CValue, so the VM cannot " + what + " through it; run the program with `kigumi build`")
		}
		return hd
	}
	h["mem.volatileRead"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		v := m.cvalue(handle(m, "read", a[0]))
		if v == nil {
			m.abort("this pointer addresses a CValue that was already dropped")
		}
		return m.copy(v)
	}
	h["mem.volatileWrite"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		m.replaceCValue(handle(m, "write", a[0]), m.copy(deref(a[1])))
		return unitObj
	}
	for _, key := range []string{"volatileReadAt", "volatileWriteAt", "atomicLoad", "atomicStore", "atomicExchange", "atomicCompareExchange", "atomicAdd", "atomicSub", "atomicAnd", "atomicOr", "atomicXor"} {
		h["mem."+key] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
			m.abort("`" + key + "` is lowered at its call site and cannot run as a function value")
			return nil
		}
	}
}

// A register nobody wrote reads as zero.
func (m *Machine) volatileReadAt(addr int64) *obj {
	m.sh.store.Lock()
	v := m.sh.mmio[uint64(addr)]
	m.sh.store.Unlock()
	if v != nil {
		return m.withTaint(scalarCopy(v), m.taintOf(v))
	}
	t := m.siteFn.Locals[m.site.Dst].Type
	if t == sem.TyBool {
		return mkBool(false)
	}
	return mkInt(0, m.numKind(t))
}

func (m *Machine) volatileWriteAt(addr int64, v *obj) {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	if m.sh.mmio == nil {
		m.sh.mmio = map[uint64]*obj{}
	}
	m.sh.mmio[uint64(addr)] = m.withTaint(scalarCopy(v), m.taintOf(v))
}
