package asmtable

func Lookup(name string) (*Architecture, bool) {
	arch, ok := asmArches[name]
	return arch, ok
}

func (a *Architecture) PointerBits() int { return a.ptrBits }

// FloatBits is the widest floating-point register's width: the only width a
// number outside the general registers can take in that register file.
func (a *Architecture) FloatBits() int {
	best := 0
	for _, r := range a.regs {
		if r.Float && r.Bits > best {
			best = r.Bits
		}
	}
	return best
}

func (a *Architecture) Register(name string) (Register, bool) {
	reg, ok := a.regs[name]
	return reg, ok
}
