package vm

func (m *Machine) unop(op string, a *obj) *obj {
	a = deref(a)
	switch op[0] {
	case '!':
		return mkBool(!a.b)
	case '-':
		if a.k == kFloat {
			return m.newFloat(-a.fl(), a.nkind())
		}
		if is128(a.nkind()) {
			return m.unop128('-', a)
		}
		return m.checked(-a.i, a.nkind(), a.i == minOf(a.nkind()))
	case '~':
		if is128(a.nkind()) {
			return m.unop128('~', a)
		}
		return m.newInt(truncateTo(^a.i, a.nkind()), a.nkind())
	}
	m.abort("unsupported unary " + op)
	return nil
}

func (m *Machine) cast(a *obj, nk int) *obj {
	a = deref(a)
	if nk&512 != 0 {
		if a.k == kFloat {
			return m.newFloat(a.fl(), nk)
		}
		if is128(a.nkind()) {
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
