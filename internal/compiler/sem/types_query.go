package sem

func (tt *TypeTable) IsOption(t TypeID) (TypeID, bool) {
	n := tt.nodes[t]
	if n.Kind == KNamed && n.Ent == tt.optionEnt && tt.optionEnt != 0 {
		return n.Args[0], true
	}
	return 0, false
}

func (tt *TypeTable) IsResult(t TypeID) (TypeID, TypeID, bool) {
	n := tt.nodes[t]
	if n.Kind == KNamed && n.Ent == tt.resultEnt && tt.resultEnt != 0 {
		return n.Args[0], n.Args[1], true
	}
	return 0, 0, false
}

func (tt *TypeTable) IsInteger(t TypeID) bool { return t >= TyI8 && t <= TyIsize }

func (tt *TypeTable) IsFloat(t TypeID) bool { return t == TyF32 || t == TyF64 }

func (tt *TypeTable) IsNumeric(t TypeID) bool { return tt.IsInteger(t) || tt.IsFloat(t) }

func (tt *TypeTable) IsSigned(t TypeID) bool {
	return (t >= TyI8 && t <= TyI128) || t == TyIsize || tt.IsFloat(t)
}

// Width is the bit width of a numeric type; usize and isize stay 64 bits on
// every target, which is the width their arithmetic and literals use. Only
// the C boundary narrows them to the target's pointer width (CWidth), where
// checkCRange traps a value that does not fit size_t or ptrdiff_t.
func (tt *TypeTable) Width(t TypeID) int { return tt.Bits(t, 64) }

// PtrBits is the target pointer width.
func (tt *TypeTable) PtrBits() int { return tt.ptrBits }

// CWidth is the width of a numeric type in C: usize and isize are size_t and
// ptrdiff_t, as wide as a pointer.
func (tt *TypeTable) CWidth(t TypeID) int { return tt.Bits(t, tt.ptrBits) }

// CAlign is the alignment in bytes of a numeric type inside a C struct on the target.
func (tt *TypeTable) CAlign(t TypeID) int {
	if tt.CWidth(t) == 64 {
		return tt.align64
	}
	return tt.CWidth(t) / 8
}

// Bits returns the width of a fixed-width numeric type; usize and isize take
// the given pointer width.
func (tt *TypeTable) Bits(t TypeID, ptrBits int) int {
	switch t {
	case TyI8, TyU8:
		return 8
	case TyI16, TyU16:
		return 16
	case TyI32, TyU32, TyF32:
		return 32
	case TyI64, TyU64, TyF64:
		return 64
	case TyI128, TyU128:
		return 128
	case TyUsize, TyIsize:
		return ptrBits
	}
	return 0
}
