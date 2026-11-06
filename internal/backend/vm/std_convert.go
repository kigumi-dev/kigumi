package vm

import (
	"math"

	"kigumi/internal/compiler/sem"
)

func inRange(v int64, s, d int) bool {
	if !isSigned(s) && v < 0 {
		return false
	}
	if isSigned(d) {
		return v >= minOf(d) && v <= int64(maxOf(d))
	}
	return v >= 0 && (bitsOf(d) == 64 || uint64(v) <= maxOf(d))
}

// convert is the prelude's numeric `toX` and `truncate`: the target kind comes
// from the callee's return type, or from the call's result for `truncate`.
func (m *Machine) convert(a0 *obj) *obj {
	tt := m.r.Types
	target := tt.Node(m.r.Fn(m.site.Ent).Sig).Elem
	if m.r.IsTruncate(m.site.Ent) {
		target = m.siteFn.Locals[m.site.Dst].Type
	}
	checked := false
	if n := tt.Node(target); n.Kind == sem.KNamed && n.Ent == tt.OptionEnt() {
		target, checked = n.Args[0], true
	}
	if target == sem.TyChar {
		return mkChar(rune(a0.i))
	}
	if !tt.IsNumeric(target) {
		return nil
	}
	dnk := tt.Width(target)
	switch {
	case tt.IsFloat(target):
		dnk |= 512
	case tt.IsSigned(target):
		dnk |= 256
	}
	if a0.k == kChar {
		return mkInt(int64(a0.ch()), dnk)
	}
	if is128(dnk) || a0.k == kInt && is128(a0.nkind()) {
		return m.convertWide(a0, dnk, checked)
	}
	if dnk&512 != 0 {
		if a0.k == kFloat {
			return mkFloat(a0.fl(), dnk)
		}
		if bitsOf(dnk) == 32 {
			// Round directly to f32; going through f64 rounds twice at the
			// midpoint and can land one ULP low.
			if isSigned(a0.nkind()) {
				return mkFloat(float64(float32(a0.i)), dnk)
			}
			return mkFloat(float64(float32(uint64(a0.i))), dnk)
		}
		if isSigned(a0.nkind()) {
			return mkFloat(float64(a0.i), dnk)
		}
		return mkFloat(float64(uint64(a0.i)), dnk)
	}
	if a0.k == kFloat {
		x := math.Trunc(a0.fl())
		if math.IsNaN(x) || x < -9223372036854775808.0 || x >= 9223372036854775808.0 {
			return m.none()
		}
		return m.some(mkInt(int64(x), dnk))
	}
	if !checked {
		return mkInt(truncateTo(a0.i, dnk), dnk)
	}
	if !inRange(a0.i, a0.nkind(), dnk) {
		return m.none()
	}
	return m.some(mkInt(a0.i, dnk))
}
