package sem

import "kigumi/internal/compiler/syntax"

// instantiateRequirement gives a requirement's own type parameters fresh variables at each call, or the explicit ones.
func (c *checker) instantiateRequirement(n syntax.NodeID, req EntityID, sig TypeID, typeArgs []TypeID) (TypeID, map[EntityID]TypeID) {
	own := c.r.Fn(req).TypeParams
	if len(typeArgs) > 0 && len(typeArgs) != len(own) {
		c.errAt(n, cTypeArgCount, c.r.Entities[req].Name, len(own), plural(len(own)), len(typeArgs))
		typeArgs = nil
	}
	if len(own) == 0 {
		return sig, nil
	}
	subst := map[EntityID]TypeID{}
	inst := make([]TypeID, len(own))
	for i, p := range own {
		switch {
		case i < len(typeArgs):
			inst[i] = typeArgs[i]
		case c.r.typeParam(p).IsLifetime:
			inst[i] = c.freshLifetime(n, p)
		default:
			inst[i] = c.vars.fresh(n, c.r.Entities[p].Name)
		}
		subst[p] = inst[i]
	}
	c.deferConstraints(n, own, inst, subst)
	c.hintFnParams(own, inst, subst)
	return c.r.Types.Subst(sig, subst), subst
}

// requirementConstArgs materializes the hidden const args a requirement call
// passes to its witness. They are the requirement's own const params, resolved
// by checkArgs from the call's arguments; a witness bind cannot fix them
// because they belong to each call.
func (c *checker) requirementConstArgs(n syntax.NodeID, req EntityID, subst map[EntityID]TypeID) []ConstArg {
	var out []ConstArg
	for _, p := range c.r.constParamsOf(req) {
		t := subst[p]
		if t == 0 {
			continue
		}
		if a := c.constArgFromType(n, c.vars.resolve(t)); a.Valid() {
			out = append(out, a)
		}
	}
	return out
}

func (c *checker) hasRequirement(t TypeID, name string) bool {
	node := c.r.Types.Node(t)
	switch node.Kind {
	case KParam:
		for _, con := range c.r.typeParam(node.Ent).Constraints {
			if con.Kind == CIface && c.r.requirement(c.r.Types.Node(con.Type).Ent, name) != 0 {
				return true
			}
		}
	case KIface:
		return c.r.requirement(node.Ent, name) != 0
	}
	return false
}
