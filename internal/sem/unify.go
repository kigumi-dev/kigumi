package sem

// unify makes a and b equal, binding variables, and rolls back on failure.
// Untyped literals unify with a variable by making it a pending literal.
func (v *varStore) unify(a, b TypeID) bool {
	return v.unifyMode(a, b, false)
}

// unifyIgnoringEscaping is unify without the fnEscaping bit at any depth; checkEscapeArg judges that bit.
func (v *varStore) unifyIgnoringEscaping(a, b TypeID) bool {
	return v.unifyMode(a, b, true)
}

func (v *varStore) unifyMode(a, b TypeID, ignoreEscaping bool) bool {
	mark := len(v.trail)
	if v.unifyInner(a, b, ignoreEscaping) {
		return true
	}
	for len(v.trail) > mark {
		i := v.trail[len(v.trail)-1]
		v.trail = v.trail[:len(v.trail)-1]
		v.binding[i] = 0
	}
	return false
}

func (v *varStore) unifyInner(a, b TypeID, ignoreEscaping bool) bool {
	a, b = v.resolve(a), v.resolve(b)
	if a == b {
		return true
	}
	tt := v.r.Types
	na, nb := tt.Node(a), tt.Node(b)
	if na.Kind == KPoison || nb.Kind == KPoison {
		return true
	}
	if nb.Kind == KVar && na.Kind != KVar {
		a, b, na, nb = b, a, nb, na
	}
	if na.Kind == KVar {
		return v.bindVar(na.Var, b, nb)
	}
	if na.Kind == KUntyped && tt.IsNumeric(b) || nb.Kind == KUntyped && tt.IsNumeric(a) {
		return false
	}
	fa, fb := na.Flags, nb.Flags
	va, vb := na.Var, nb.Var
	if ignoreEscaping && na.Kind == KFn && nb.Kind == KFn {
		fa, fb, va, vb = fa&^fnEscaping, fb&^fnEscaping, 0, 0
	}
	if na.Kind != nb.Kind || fa != fb || na.Ent != nb.Ent || va != vb || len(na.Args) != len(nb.Args) {
		return false
	}
	if !v.unifyInner(na.Elem, nb.Elem, ignoreEscaping) {
		return false
	}
	for i := range na.Args {
		if !v.unifyInner(na.Args[i], nb.Args[i], ignoreEscaping) {
			return false
		}
	}
	return true
}

func (v *varStore) bindVar(i uint32, t TypeID, n typeNode) bool {
	tt := v.r.Types
	t = tt.WithoutEscaping(t)
	switch {
	case n.Kind == KVar:
		if v.pending[i] != 0 && v.pending[n.Var] == 0 {
			v.pending[n.Var] = v.pending[i]
			v.lits[n.Var] = append(v.lits[n.Var], v.lits[i]...)
		} else if v.pending[i] == 2 && v.pending[n.Var] == 1 {
			v.pending[n.Var] = 2
		}
		v.bind(i, t)
		return true
	case n.Kind == KUntyped:
		kind := uint8(1)
		if t == TyUntypedFloat {
			kind = 2
		}
		if v.pending[i] < kind {
			v.pending[i] = kind
		}
		return true
	case v.pending[i] != 0:
		if !tt.IsNumeric(t) || (v.pending[i] == 2 && tt.IsInteger(t)) {
			return false
		}
		v.bind(i, t)
		return true
	}
	if tt.Contains(t, tt.Var(i)) {
		return false
	}
	v.bind(i, t)
	return true
}

// unifiable reports whether unify would succeed, without binding.
func (v *varStore) unifiable(a, b TypeID) bool {
	mark := len(v.trail)
	ok := v.unifyInner(a, b, false)
	for len(v.trail) > mark {
		i := v.trail[len(v.trail)-1]
		v.trail = v.trail[:len(v.trail)-1]
		v.binding[i] = 0
	}
	return ok
}
