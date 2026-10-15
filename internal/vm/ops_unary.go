package vm

func (m *Machine) unop(op string, a *obj) *obj {
	a = deref(a)
	switch op[0] {
	case '!':
		return mkBool(!a.b)
	case '-':
		if a.k == kFloat {
			return m.newFloat(-a.f, a.nk)
		}
		if is128(a.nk) {
			return m.unop128('-', a)
		}
		return m.checked(-a.i, a.nk, a.i == minOf(a.nk))
	case '~':
		if is128(a.nk) {
			return m.unop128('~', a)
		}
		return m.newInt(truncateTo(^a.i, a.nk), a.nk)
	}
	m.abort("unsupported unary " + op)
	return nil
}

func (m *Machine) cast(a *obj, nk int) *obj {
	a = deref(a)
	if nk&512 != 0 {
		if a.k == kFloat {
			return m.newFloat(a.f, nk)
		}
		if is128(a.nk) {
			neg, mag := signMag(a)
			return m.newFloat(wideToFloat(neg, mag, 53), nk)
		}
		return m.newFloat(float64(a.i), nk)
	}
	if is128(nk) && a.k == kInt {
		return mkWide(widen(a), nk)
	}
	return m.newInt(a.i, nk)
}
