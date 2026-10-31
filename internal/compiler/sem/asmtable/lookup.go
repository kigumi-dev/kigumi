package asmtable

func Lookup(name string) (*Architecture, bool) {
	arch, ok := asmArches[name]
	return arch, ok
}

func (a *Architecture) PointerBits() int { return a.ptrBits }

func (a *Architecture) Register(name string) (Register, bool) {
	reg, ok := a.regs[name]
	return reg, ok
}
