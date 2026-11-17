package vm

import (
	"slices"
	"strings"

	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func numericConversion(key string) bool {
	for _, g := range opdef.Registry.Bodiless {
		if g.Name == "NUMERIC_CONVERSIONS" {
			return slices.Contains(g.Keys, key) || slices.ContainsFunc(g.Prefixes, func(p string) bool { return strings.HasPrefix(key, p) })
		}
	}
	return false
}

func (m *Machine) conversionLabel(ent sem.EntityID, args []*obj, r *obj) (*obj, bool) {
	if ent == 0 || len(args) == 0 || m.r.Entity(ent).Kind != sem.EntFn {
		return r, false
	}
	info := m.r.Fn(ent)
	if info.Recv == sem.RecvNone || len(info.Params) != 0 {
		return r, false
	}
	src, dst := m.r.Entity(info.SelfParam).Type, m.r.Types.Node(info.Sig).Elem
	opt := m.r.Types.Kind(dst) == sem.KNamed && m.r.Types.Node(dst).Ent == m.r.Types.OptionEnt()
	if opt {
		dst = m.r.Types.Node(dst).Args[0]
	}
	if !m.r.Types.IsInteger(src) || !m.r.Types.IsInteger(dst) {
		return r, false
	}
	ta := m.taintOf(args[0])
	if !ta.tainted() {
		return r, true
	}
	sk, dk := m.numKind(src), m.numKind(dst)
	value := maskTo(signExtendShadow(ta, sk), dk)
	if !opt {
		return m.withTaint(r, value), true
	}
	return m.withLabel(r, label{elem: value, shape: outOfRange(ta, sk, dk)}), true
}

func outOfRange(t shadow, sk, dk int) bool {
	if t.coarse {
		return true
	}
	srcW, cut := bitsOf(sk), bitsOf(dk)
	if isSigned(dk) {
		cut--
	}
	dep := bitRange(cut, srcW)
	if isSigned(sk) && !isSigned(dk) {
		sign := bitRange(srcW-1, srcW)
		dep.lo, dep.hi = dep.lo|sign.lo, dep.hi|sign.hi
	}
	return t.lo&dep.lo != 0 || t.hi&dep.hi != 0
}

func bitRange(from, to int) wide {
	var w wide
	for i := max(from, 0); i < min(to, 128); i++ {
		if i < 64 {
			w.lo |= 1 << uint(i)
		} else {
			w.hi |= 1 << uint(i-64)
		}
	}
	return w
}
