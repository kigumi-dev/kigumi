package sem

func (tt *TypeTable) Fn(params []TypeID, ret TypeID, eff Effects, variadic bool) TypeID {
	return tt.fnOf(params, ret, uint16(eff), variadic)
}

// FnEffectPoly builds a `pure? fn(A) -> B` effect-polymorphic type; EffPure in eff is ignored.
func (tt *TypeTable) FnEffectPoly(params []TypeID, ret TypeID, eff Effects, variadic bool) TypeID {
	return tt.fnOf(params, ret, uint16(eff&^EffPure)|fnEffectPoly, variadic)
}

// FnEscaping builds an `escaping fn(A) -> B` parameter type; effectPoly composes it with FnEffectPoly.
func (tt *TypeTable) FnEscaping(params []TypeID, ret TypeID, eff Effects, variadic, effectPoly bool) TypeID {
	flags := uint16(eff) | fnEscaping
	if effectPoly {
		flags = flags&^uint16(EffPure) | fnEffectPoly
	}
	return tt.fnOf(params, ret, flags, variadic)
}

func (tt *TypeTable) fnOf(params []TypeID, ret TypeID, flags uint16, variadic bool) TypeID {
	if variadic {
		flags |= fnVariadic
	}
	return tt.Intern(typeNode{Kind: KFn, Flags: flags, Elem: ret, Args: params})
}

// FnIsEscaping reports whether t is a KFn type written `escaping fn(...)`.
func (tt *TypeTable) FnIsEscaping(t TypeID) bool {
	n := tt.Node(t)
	return n.Kind == KFn && n.Flags&fnEscaping != 0
}

// WithEscaping returns the plain KFn type t with fnEscaping set, as if written `escaping fn(...)`.
func (tt *TypeTable) WithEscaping(t TypeID) TypeID {
	n := tt.Node(t)
	if n.Flags&fnEscaping != 0 {
		return t
	}
	n.Flags |= fnEscaping
	return tt.Intern(n)
}

// SameIgnoringEscaping is Same without the fnEscaping bit; escapeMismatch checks that bit on its own.
func (tt *TypeTable) SameIgnoringEscaping(a, b TypeID) bool {
	if a == b {
		return true
	}
	if a == 0 || b == 0 {
		return false
	}
	na, nb := tt.Node(a), tt.Node(b)
	fa, fb := na.Flags, nb.Flags
	if na.Kind == KFn && nb.Kind == KFn {
		fa, fb = fa&^fnEscaping, fb&^fnEscaping
	}
	if na.Kind != nb.Kind || na.Ent != nb.Ent || na.Var != nb.Var || fa != fb || len(na.Args) != len(nb.Args) {
		return false
	}
	if !tt.SameIgnoringEscaping(na.Elem, nb.Elem) {
		return false
	}
	for i := range na.Args {
		if !tt.SameIgnoringEscaping(na.Args[i], nb.Args[i]) {
			return false
		}
	}
	return true
}

func (tt *TypeTable) FnCPtr(params []TypeID, ret TypeID, variadic bool) TypeID {
	flags := fnCAbi
	if variadic {
		flags |= fnCVariadic
	}
	return tt.Intern(typeNode{Kind: KFn, Flags: flags, Elem: ret, Args: params})
}

// IsCFn reports whether t is a C function pointer type.
func (tt *TypeTable) IsCFn(t TypeID) bool {
	n := tt.Node(t)
	return n.Kind == KFn && n.Flags&fnCAbi != 0
}

func (tt *TypeTable) Ref(t TypeID, mut bool) TypeID {
	return tt.Intern(typeNode{Kind: KRef, Flags: mutFlag(mut), Elem: t})
}

// RefMut reports whether t is `&mut U` (false for a shared `&U` or a
// non-Ref type).
func (tt *TypeTable) RefMut(t TypeID) bool {
	n := tt.Node(t)
	return n.Kind == KRef && n.Flags&flagMut != 0
}

func (tt *TypeTable) Ptr(t TypeID, mut bool) TypeID {
	return tt.Intern(typeNode{Kind: KPtr, Flags: mutFlag(mut), Elem: t})
}

func mutFlag(mut bool) uint16 {
	if mut {
		return flagMut
	}
	return 0
}
