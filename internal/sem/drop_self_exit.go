package sem

// selfExitDrop records the scope-exit drop of an unconsumed `move self`; inside the type's own drop only its parts drop.
func (c *checker) selfExitDrop(fn EntityID) {
	info := c.r.Fn(fn)
	sp := info.SelfParam
	if sp == 0 || info.Recv != RecvMove || c.consumed[sp] {
		return
	}
	self := &c.r.Entities[sp]
	n := c.r.Types.Node(self.Type)
	if n.Kind != KNamed || c.r.typeDecl(n.Ent).Drop != fn {
		c.implicitDrop(sp)
		return
	}
	decl := c.r.typeDecl(n.Ent)
	subst := map[EntityID]TypeID{}
	for i, p := range decl.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, f := range decl.Fields {
		if t := c.r.Types.Subst(c.r.Entities[f].Type, subst); c.r.Types.Kind(t) == KClosure || !c.r.isCopy(t) {
			c.edge(EffectEdge{Kind: EdgeDrop, Type: t, Node: self.Node})
		}
	}
}
