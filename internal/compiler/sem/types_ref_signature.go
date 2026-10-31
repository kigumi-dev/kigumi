package sem

import "kigumi/internal/compiler/syntax"

// Callable signatures retain the tag that ordinary reference declarations store on their parameter entity.
func (tt *TypeTable) WithRefLifetime(t, lifetime TypeID) TypeID {
	n := tt.Node(t)
	if n.Kind != KRef || n.Lifetime == lifetime {
		return t
	}
	n.Lifetime = lifetime
	return tt.Intern(n)
}

func (tt *TypeTable) RefLifetime(t TypeID) TypeID {
	if n := tt.Node(t); n.Kind == KRef {
		return n.Lifetime
	}
	return 0
}

func (tt *TypeTable) ExactRefParams(a, b TypeID) bool {
	left, right := tt.Node(a).Args, tt.Node(b).Args
	if len(left) != len(right) {
		return false
	}
	for i, t := range left {
		if (tt.RefLifetime(t) == 0) != (tt.RefLifetime(right[i]) == 0) {
			return false
		}
	}
	return true
}

func (tt *TypeTable) FnParamKeepsRef(sig TypeID, i int) bool {
	n := tt.Node(sig)
	return n.Kind == KFn && i >= 0 && i < len(n.Args) && tt.Kind(n.Args[i]) == KRef &&
		tt.RefLifetime(n.Args[i]) != 0
}

func (r *Result) refSignature(sig TypeID, params []EntityID) TypeID {
	n := r.Types.Node(sig)
	args := append([]TypeID(nil), n.Args...)
	for i, p := range params {
		if lt := r.Local(p).Lifetime; lt != 0 {
			args[i] = r.Types.WithRefLifetime(args[i], r.Types.Param(lt))
		}
	}
	n.Args = args
	return r.Types.Intern(n)
}

func (r *Result) refTypeSlot(f FileID, scope ScopeID, node syntax.NodeID, typ TypeID) TypeID {
	for r.tree(f).Kind(node) == syntax.Paren {
		node = r.tree(f).View(node).AsParen().Inner().ID()
	}
	if lt := r.explicitLifetimeOf(f, scope, node); lt != 0 {
		return r.Types.WithRefLifetime(typ, r.Types.Param(lt))
	}
	return typ
}
