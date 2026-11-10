package vm

// selectShadow labels (mask ? a : b) by bit: a bit where a and b agree does not depend on the mask.
func (m *Machine) selectShadow(mask, a, b *obj) shadow {
	tm, ta, tb := m.taintOf(mask), m.taintOf(a), m.taintOf(b)
	av, bv := widen(deref(a)), widen(deref(b))
	eq := wide{lo: ^(av.lo ^ bv.lo), hi: ^(av.hi ^ bv.hi)}
	t := shadow{
		lo: (ta.lo|tb.lo)&eq.lo | (tm.lo|ta.lo|tb.lo)&^eq.lo,
		hi: (ta.hi|tb.hi)&eq.hi | (tm.hi|ta.hi|tb.hi)&^eq.hi,
	}
	if tm.coarse || ta.coarse || tb.coarse {
		t.coarse = true
	}
	return t
}

// binopShadow is bit-precise for &, | and ^ and for shifts by a public count; an operand's coarse label, which
// tracks no bits, joins every result, so only a bit-tracked secret can be cleared by a public mask.
func (m *Machine) binopShadow(c bop, op string, a, b *obj) shadow {
	ta, tb := m.taintOf(a), m.taintOf(b)
	if !ta.tainted() && !tb.tainted() {
		return shadow{}
	}
	s := m.bitsOfBinop(c, ta, tb, deref(a), deref(b))
	s.coarse = s.coarse || ta.coarse || tb.coarse
	return s
}

func (m *Machine) bitsOfBinop(c bop, ta, tb shadow, da, db *obj) shadow {
	switch c {
	case bopEq, bopNe, bopLt, bopLe, bopGt, bopGe, bopOther:
		return secretCoarse
	case bopAnd, bopOr, bopXor:
		if da.k == kInt && db.k == kInt {
			return maskTo(m.bitShadow(c, ta, tb, widen(da), widen(db)), da.nkind())
		}
		return secretCoarse
	case bopShl, bopShr:
		return maskTo(m.shiftShadow(c, ta, db, da.nkind()), da.nkind())
	}
	return fullShadow(da.nkind())
}

// bitShadow is MSan's rule: a result bit depends on an operand bit only where the other operand lets it through.
func (m *Machine) bitShadow(c bop, ta, tb shadow, av, bv wide) shadow {
	switch c {
	case bopAnd:
		return shadow{lo: ta.lo&bv.lo | tb.lo&av.lo, hi: ta.hi&bv.hi | tb.hi&av.hi}
	case bopOr:
		return shadow{lo: ta.lo&^bv.lo | tb.lo&^av.lo, hi: ta.hi&^bv.hi | tb.hi&^av.hi}
	default:
		return shadow{lo: ta.lo | tb.lo, hi: ta.hi | tb.hi}
	}
}

// shiftShadow shifts a bit label with its value by a public count.
func (m *Machine) shiftShadow(c bop, ta shadow, count *obj, nk int) shadow {
	n := uint(uint64(count.i))
	if c == bopShl {
		if is128(nk) {
			return wideShadow(shlW(wide{ta.lo, ta.hi}, n))
		}
		return shadow{lo: ta.lo << n}
	}
	if is128(nk) {
		s := wideShadow(shrW(wide{ta.lo, ta.hi}, n))
		if isSigned(nk) && ta.hi>>63 != 0 && n > 0 {
			s.hi |= ^uint64(0) << (64 - n)
		}
		return s
	}
	s := shadow{lo: ta.lo >> n}
	if isSigned(nk) && ta.lo>>63 != 0 && n > 0 {
		s.lo |= ^uint64(0) << (64 - n)
	}
	return s
}

func wideShadow(w wide) shadow { return shadow{lo: w.lo, hi: w.hi} }

func (m *Machine) unopShadow(op string, a *obj) shadow {
	ta := m.taintOf(a)
	if !ta.tainted() {
		return shadow{}
	}
	da := deref(a)
	switch {
	case op[0] == '!' || da.k != kInt:
		return secretCoarse
	case op[0] == '~':
		return maskTo(ta, da.nkind())
	}
	return fullShadow(da.nkind())
}

// castShadow carries a bit label across a numeric conversion.
func (m *Machine) castShadow(a *obj, nk int) shadow {
	ta := m.taintOf(a)
	if !ta.tainted() {
		return shadow{}
	}
	if nk&512 != 0 {
		return secretCoarse
	}
	if da := deref(a); da.k != kInt {
		return secretCoarse
	}
	return maskTo(ta, nk)
}
