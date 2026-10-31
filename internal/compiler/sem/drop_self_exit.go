package sem

import "kigumi/internal/compiler/syntax"

// selfExitDrop records the scope-exit drop of an unconsumed `move self`: what is left of it after the fields the body moved out, and inside the type's own drop only its parts.
func (c *checker) selfExitDrop(fn EntityID) {
	info := c.r.Fn(fn)
	sp := info.SelfParam
	if sp == 0 || info.Recv != RecvMove || c.consumed.whole[sp] {
		return
	}
	self := &c.r.Entities[sp]
	n := c.r.Types.Node(self.Type)
	if n.Kind == KNamed && c.r.typeDecl(n.Ent).Drop == fn {
		c.dropParts(n, nil, self.Node)
		return
	}
	c.dropRemaining(self.Type, c.consumed.parts[sp], self.Node)
}

// dropRemaining charges the drop of a value of type t less the field paths moved out of it; a type with its own drop is charged whole.
func (c *checker) dropRemaining(t TypeID, moved [][]EntityID, node syntax.NodeID) {
	n := c.r.Types.Node(t)
	if len(moved) == 0 || n.Kind != KNamed || c.r.typeDecl(n.Ent).Drop != 0 {
		if c.r.Types.Kind(t) == KClosure || !c.r.isCopy(t) {
			c.edge(EffectEdge{Kind: EdgeDrop, Type: t, Node: node})
		}
		return
	}
	c.dropParts(n, moved, node)
}

func (c *checker) dropParts(n typeNode, moved [][]EntityID, node syntax.NodeID) {
	decl := c.r.typeDecl(n.Ent)
	subst := map[EntityID]TypeID{}
	for i, p := range decl.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, f := range decl.Fields {
		var inner [][]EntityID
		whole := false
		for _, m := range moved {
			switch {
			case m[0] != f:
			case len(m) == 1:
				whole = true
			default:
				inner = append(inner, m[1:])
			}
		}
		if !whole {
			c.dropRemaining(c.r.Types.Subst(c.r.Entities[f].Type, subst), inner, node)
		}
	}
}
