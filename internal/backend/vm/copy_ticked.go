package vm

import (
	"sync/atomic"

	"kigumi/internal/compiler/mir"
)

// copier copies a value as one of native's two deep copies does, allocation for allocation: copy_try
// (try set: exact array stores, an owned Bytes buffer is refusable, a refusal is reported) and rt_copy (a refusal aborts).
type copier struct {
	m   *Machine
	try bool
}

func (m *Machine) copyTry(v *obj) (*obj, bool) { return copier{m, true}.value(v) }

func (m *Machine) copyShared(f *mir.Func, in *mir.Inst, ic *instCode, v *obj) *obj {
	if m.opts.FaultInjectAt <= 0 || f.IsDirect(in.Dst) || ic.inlineCopy {
		return m.copy(v)
	}
	return m.copyPanicking(v)
}

func (m *Machine) copyPanicking(v *obj) *obj {
	out, ok := copier{m, false}.value(v)
	if !ok {
		m.abort("out of memory")
	}
	return out
}

func (m *Machine) copiedResult(key string, v *obj) *obj {
	plan := tryPlans[key]
	if m.inlineResultCopy(key) {
		return m.copy(v)
	}
	if plan.copy == copySome && m.site != nil && m.site.DPrim != mir.DPNone {
		return m.some(retain(v))
	}
	if plan.copy == copyOk {
		out, ok := m.copyTry(v)
		if !ok {
			return m.allocError()
		}
		return m.okPlanned(key, out)
	}
	out := m.copyPanicking(v)
	if plan.copy == copySome {
		if m.wrapRefused(1) {
			m.release(out)
			m.abort("out of memory")
		}
		return m.some(out)
	}
	return out
}

func (m *Machine) inlineResultCopy(key string) bool {
	return tryPlans[key].copy == copyElements && m.site != nil && m.siteFn != nil && m.siteFn.IsDirect(m.site.Dst)
}

// ticks takes n refusable allocations, stopping at the first refused one.
func (m *Machine) ticks(n int) bool {
	for range n {
		if m.faultTick() {
			return true
		}
	}
	return false
}

func (c copier) value(v *obj) (*obj, bool) {
	if v == nil || atomic.LoadInt32(&v.rc) < 0 {
		return v, true
	}
	m := c.m
	switch v.k {
	case kRecord:
		if m.isResource(v.ent) {
			return retain(v), true
		}
		return c.shell(v, 2, func() *obj { return mkRecord(v.ent, v.fields) }, func(i int) bool { return m.borrowField(v, i) })
	case kVariant:
		if len(v.fields) == 0 {
			return m.copy(v), true
		}
		return c.shell(v, 2, func() *obj { return mkVariant(v.ent, v.fields) }, func(int) bool { return false })
	case kArray:
		n := 1
		if len(v.fields) > 0 {
			n = 2
		}
		out, ok := c.shell(v, n, func() *obj { return mkArray(v.fields) }, func(int) bool { return false })
		if ok && c.try {
			out.i = int64(len(out.fields))
		}
		return out, ok
	case kBytes:
		n := 1
		if c.try {
			n = 2
		}
		if m.ticks(n) {
			return nil, false
		}
		return m.copy(v), true
	case kBox:
		if m.ticks(1) {
			return nil, false
		}
		inner, ok := c.value(v.inner)
		if !ok {
			return nil, false
		}
		out := newObj(kBox)
		out.i, out.inner = v.i, inner
		m.chargeMemory(out)
		return out, true
	}
	return m.copy(v), true
}

// shell takes the allocations of the copy's own header and slots, then copies each field the copy owns.
func (c copier) shell(src *obj, allocs int, build func() *obj, borrowed func(int) bool) (*obj, bool) {
	if c.m.ticks(allocs) {
		return nil, false
	}
	out := build()
	for i := range out.fields {
		if borrowed(i) {
			continue
		}
		f, ok := c.value(src.fields[i])
		if !ok {
			for j := i; j < len(out.fields); j++ {
				if !borrowed(j) {
					out.fields[j] = nil
				}
			}
			c.m.release(out)
			return nil, false
		}
		out.fields[i] = f
	}
	c.m.chargeMemory(out)
	return out, true
}
