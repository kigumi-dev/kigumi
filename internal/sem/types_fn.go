package sem

import "strings"

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

func (tt *TypeTable) WithoutEscaping(t TypeID) TypeID {
	n := tt.Node(t)
	if n.Kind != KFn || n.Flags&fnEscaping == 0 {
		return t
	}
	n.Flags &^= fnEscaping
	return tt.Intern(n)
}

func (tt *TypeTable) WithEscParams(t TypeID, set string) TypeID {
	n := tt.Node(t)
	idx := tt.internEscSet(set)
	if n.Kind != KFn || n.Var == idx {
		return t
	}
	n.Var = idx
	return tt.Intern(n)
}

// Once F is a fn type the `escaping F` mark moves onto its escaping bit, so the instance spells the same as a written `escaping fn(...)`.
func (tt *TypeTable) absorbEscMarks(n typeNode) typeNode {
	if n.Kind != KFn || n.Var == 0 {
		return n
	}
	args := append([]TypeID(nil), n.Args...)
	rest := make([]byte, len(tt.escSets[n.Var-1]))
	for i, a := range args {
		switch {
		case !tt.escMarked(n.Var, i):
		case tt.Kind(a) == KFn:
			args[i] = tt.WithEscaping(a)
		default:
			rest[i/8] |= 1 << (i % 8)
		}
	}
	n.Args = args
	n.Var = tt.internEscSet(strings.TrimRight(string(rest), "\x00"))
	return n
}

func (tt *TypeTable) internEscSet(set string) uint32 {
	if set == "" {
		return 0
	}
	if idx, ok := tt.escIndex[set]; ok {
		return idx
	}
	if tt.escIndex == nil {
		tt.escIndex = map[string]uint32{}
	}
	tt.escSets = append(tt.escSets, set)
	idx := uint32(len(tt.escSets))
	tt.escIndex[set] = idx
	return idx
}

func (tt *TypeTable) escMarked(idx uint32, i int) bool {
	if idx == 0 {
		return false
	}
	set := tt.escSets[idx-1]
	return i/8 < len(set) && set[i/8]&(1<<(i%8)) != 0
}

func (tt *TypeTable) FnParamEscapes(t TypeID, i int) bool {
	n := tt.Node(t)
	if n.Kind != KFn || i < 0 || i >= len(n.Args) {
		return false
	}
	return tt.escMarked(n.Var, i) || tt.FnIsEscaping(n.Args[i])
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
	va, vb := na.Var, nb.Var
	if na.Kind == KFn && nb.Kind == KFn {
		fa, fb, va, vb = fa&^fnEscaping, fb&^fnEscaping, 0, 0
	}
	if na.Kind != nb.Kind || na.Ent != nb.Ent || va != vb || fa != fb || len(na.Args) != len(nb.Args) {
		return false
	}
	if !tt.SameIgnoringEscaping(na.Lifetime, nb.Lifetime) || !tt.SameIgnoringEscaping(na.Elem, nb.Elem) {
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
