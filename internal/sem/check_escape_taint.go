package sem

import "kigumi/internal/syntax"

// mayContainFn reports whether t could hold a fn. A generic/interface slot is not assumed to, since
// checkEscapeArg/Store already reject storing a restricted value there; visited guards cycles.
func (r *Result) mayContainFn(t TypeID, visited map[TypeID]bool) bool {
	if t == 0 || visited[t] {
		return false
	}
	switch r.Types.Kind(t) {
	case KFn, KClosure, KVar:
		return true
	case KParam:
		return r.fnShapeOf(t) != 0
	case KRef, KPtr:
		return r.mayContainFn(r.Types.Node(t).Elem, visited)
	case KNamed:
		visited[t] = true
		n := r.Types.Node(t)
		decl := r.typeDecl(n.Ent)
		subst := instSubst(decl.Params, n.Args)
		switch decl.Form {
		case FormDefined:
			return r.mayContainFn(r.Types.Subst(decl.Underlying, subst), visited)
		case FormRecord, FormResource:
			for _, fld := range decl.Fields {
				if r.mayContainFn(r.Types.Subst(r.field(fld).Type, subst), visited) {
					return true
				}
			}
			return false
		case FormAdt:
			for _, v := range decl.Variants {
				for _, pt := range r.variant(v).Payload {
					if r.mayContainFn(r.Types.Subst(pt, subst), visited) {
						return true
					}
				}
			}
			return false
		default: // FormOpaque: no visible layout, but its own type arguments are
			for _, a := range n.Args {
				if r.mayContainFn(a, visited) {
					return true
				}
			}
			return false
		}
	default:
		return false
	}
}

// instSubst pairs params with args positionally, for TypeTable.Subst; nil
// when params is empty (Subst treats a nil map as the identity).
func instSubst(params []EntityID, args []TypeID) map[EntityID]TypeID {
	if len(params) == 0 {
		return nil
	}
	s := make(map[EntityID]TypeID, len(params))
	for i, p := range params {
		if i < len(args) {
			s[p] = args[i]
		}
	}
	return s
}

// taintCallResult is the one place a call's result can carry a tainted parameter onward: the callee
// may be an opaque value, witness, or generic instantiation never checked against this exact call.
func (c *checker) taintCallResult(n syntax.NodeID, ret TypeID, args []syntax.NodeID) {
	if !c.r.mayContainFn(c.vars.resolve(ret), map[TypeID]bool{}) {
		return
	}
	for _, a := range args {
		if c.t.Kind(a) == syntax.Spread || c.t.Kind(a) == syntax.NamedArg {
			a = syntax.NodeID(c.t.Nodes[a].Lhs)
		}
		if rp := c.restrictedParam(a); rp != 0 {
			c.taintedCall[n] = rp
			return
		}
	}
}
