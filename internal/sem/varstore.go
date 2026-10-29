package sem

import "kigumi/internal/syntax"

// varStore is the per-body table of inference variables: a
// union-find with bindings, pending literals and a trail for rollback.
type varStore struct {
	r       *Result
	binding []TypeID // 0 = unbound
	pending []uint8  // 0 none, 1 int, 2 float
	lits    [][]syntax.NodeID
	origin  []syntax.NodeID
	names   []string
	hint    []TypeID
	// lifetimeDefault is a lifetime var's settle() fallback,
	// 0 for an ordinary type-parameter variable (still cInferUnresolved).
	lifetimeDefault []TypeID
	first           int // first variable created since the last boundary
	trail           []uint32
}

func newVarStore(r *Result) *varStore { return &varStore{r: r} }

func (v *varStore) fresh(origin syntax.NodeID, name string) TypeID {
	v.binding = append(v.binding, 0)
	v.pending = append(v.pending, 0)
	v.lits = append(v.lits, nil)
	v.origin = append(v.origin, origin)
	v.names = append(v.names, name)
	v.hint = append(v.hint, 0)
	v.lifetimeDefault = append(v.lifetimeDefault, 0)
	return v.r.Types.Var(uint32(len(v.binding) - 1))
}

// fnHint is the fn shape a variable's type parameter demands through an Fn constraint.
func (v *varStore) fnHint(t TypeID) TypeID {
	if i, ok := v.index(v.resolve(t)); ok {
		return v.hint[i]
	}
	return 0
}

// poison binds every unbound variable inside t to TyPoison so that a
// reported error is not followed by an infer-unresolved error.
func (v *varStore) poison(t TypeID) {
	t = v.resolve(t)
	if t == 0 || !v.r.Types.ContainsVar(t) {
		return
	}
	n := v.r.Types.Node(t)
	if n.Kind == KVar {
		v.binding[n.Var] = TyPoison
		return
	}
	v.poison(n.Elem)
	for _, a := range n.Args {
		v.poison(a)
	}
}

func (v *varStore) boundary() { v.first = len(v.binding) }

func (v *varStore) index(t TypeID) (uint32, bool) {
	n := v.r.Types.Node(t)
	if n.Kind != KVar {
		return 0, false
	}
	return n.Var, true
}

// pendingIndex is index for an unbound variable that already carries a pending literal default.
func (v *varStore) pendingIndex(t TypeID) (uint32, bool) {
	i, ok := v.index(t)
	if !ok || v.pending[i] == 0 {
		return 0, false
	}
	return i, true
}

// resolve replaces bound variables inside t, recursively.
func (v *varStore) resolve(t TypeID) TypeID {
	if t == 0 || !v.r.Types.ContainsVar(t) {
		return t
	}
	tt := v.r.Types
	n := tt.Node(t)
	if n.Kind == KVar {
		if b := v.binding[n.Var]; b != 0 {
			return v.resolve(b)
		}
		return t
	}
	args := make([]TypeID, len(n.Args))
	changed := false
	for i, a := range n.Args {
		args[i] = v.resolve(a)
		changed = changed || args[i] != a
	}
	elem, lifetime := v.resolve(n.Elem), v.resolve(n.Lifetime)
	if !changed && elem == n.Elem && lifetime == n.Lifetime && n.Kind != KFn {
		return t
	}
	return tt.Intern(tt.absorbEscMarks(typeNode{Kind: n.Kind, Flags: n.Flags, Ent: n.Ent, Elem: elem, Lifetime: lifetime, Var: n.Var, Args: args}))
}

func (v *varStore) bind(i uint32, t TypeID) { v.trail = append(v.trail, i); v.binding[i] = t }
