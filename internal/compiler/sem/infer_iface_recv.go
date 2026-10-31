package sem

import "kigumi/internal/compiler/syntax"

// ifaceRecvEnt names the local/param an existential receiver resolves to, so
// root-bridge reachability can trace it to its own witnesses.
func (c *checker) ifaceRecvEnt(base syntax.NodeID, recv TypeID) EntityID {
	t := recv
	if c.r.Types.Kind(t) == KRef {
		t = c.r.Types.Node(t).Elem
	}
	if c.r.Types.Kind(t) != KIface {
		return 0
	}
	if c.t.Kind(base) == syntax.MemberExpr {
		if src := c.fieldWitnessRecvEnt(base); src != 0 {
			return src
		}
	}
	return c.info.Uses[base]
}

// fieldWitnessRecvEnt resolves base's (owner, field) pair: a non-mut local
// aliases the source directly for per-call-site narrowing; mut/param mint a field-param entity instead.
func (c *checker) fieldWitnessRecvEnt(base syntax.NodeID) EntityID {
	return c.r.chainFieldWitnessRecvEnt(c.f, base)
}

// fieldKeyEnt is fieldWitnessRecvEnt's gate for when the caller already has
// owner's entity (e.g. ifaceFieldArgEdge chaining) instead of a MemberExpr.
func (r *Result) fieldKeyEnt(owner, field EntityID) EntityID {
	key := fieldFnKey{Ent: owner, Field: field}
	if r.Entities[owner].Kind == EntLocal && r.Entities[owner].Flags&EfMut == 0 {
		return r.fieldWitnessOf[key]
	}
	return r.fieldParamEnt(key)
}
