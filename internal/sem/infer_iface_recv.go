package sem

import "kigumi/internal/syntax"

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

func (c *checker) fieldWitnessRecvEnt(base syntax.NodeID) EntityID {
	field := c.info.Uses[base]
	if field == 0 {
		return 0
	}
	baseExpr := syntax.NodeID(c.t.Nodes[base].Lhs)
	if c.t.Kind(baseExpr) != syntax.Ident {
		return 0
	}
	baseEnt := c.info.Uses[baseExpr]
	if baseEnt == 0 {
		return 0
	}
	return c.r.fieldWitnessOf[fieldFnKey{Ent: baseEnt, Field: field}]
}
