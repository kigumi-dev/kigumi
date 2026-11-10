package vm

import "sync/atomic"

// withTaint labels the content of a value this run produced; a shared immortal is first replaced by its label site.
func (m *Machine) withTaint(v *obj, t shadow) *obj { return m.withLabel(v, label{elem: t}) }

func (m *Machine) withLabel(v *obj, l label) *obj {
	if !m.opts.SecretSan || v == nil || !l.tainted() {
		return v
	}
	if atomic.LoadInt32(&v.rc) < 0 {
		v = m.siteOf(v).v
	}
	m.mergeLabel(v, l)
	return v
}

// inheritLabel gives a runtime copy dst the label of the value it copied.
func (m *Machine) inheritLabel(src, dst *obj) *obj {
	if !m.opts.SecretSan || src == nil || dst == nil || src == dst {
		return dst
	}
	m.mergeLabel(dst, m.labelOf(src))
	return dst
}

// deriveTaint is res read out of a value labelled from: its content depends on all of from, and a Go-built part
// inherits from's shape. res is owned by the caller; a copied scalar keeps res's own label.
func (m *Machine) deriveTaint(res *obj, from label) *obj {
	if !m.opts.SecretSan || res == nil || !from.tainted() {
		return res
	}
	derived := label{elem: from.whole(), shape: from.shape}
	if s := m.siteOf(res); s.kind == siteCopy {
		derived = derived.merge(m.labelOf(res))
		m.release(res)
		res = s.v
	}
	m.mergeLabel(res, derived)
	return res
}

// tagTest is the Bool of a variant or dynamic-type test on v, which depends only on v's shape.
func (m *Machine) tagTest(v *obj, ok bool) *obj {
	if m.opts.SecretSan && m.shapeOf(v) {
		return m.withTaint(mkBool(ok), secretCoarse)
	}
	return mkBool(ok)
}

// joinWrite joins the label of v into the content of the aggregate it was stored in; the shape stays.
func (m *Machine) joinWrite(container, v *obj) {
	if m.opts.SecretSan {
		m.addLabel(derefRaw(container), m.taintOf(v))
	}
}

// untaint clears every label from a value this run just produced (declassify).
func (m *Machine) untaint(v *obj) *obj {
	if v != nil && atomic.LoadInt32(&v.rc) >= 0 {
		m.clearLabel(v)
	}
	return v
}
