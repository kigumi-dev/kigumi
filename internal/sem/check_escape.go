package sem

import "kigumi/internal/syntax"

func (c *checker) restrictedEntity(ent EntityID) bool {
	ent = c.aliasChase(ent)
	return ent != 0 && c.r.Entities[ent].Kind == EntParam && c.r.Entities[ent].Parent == c.fn && c.r.nonEscapingParam(ent)
}

// aliasChase follows paramAlias/escapeAlias chains to the root entity, ent itself if neither has one.
func (c *checker) aliasChase(ent EntityID) EntityID {
	for {
		if alias, ok := c.paramAlias[ent]; ok {
			ent = alias
			continue
		}
		if alias, ok := c.escapeAlias[ent]; ok {
			ent = alias
			continue
		}
		return ent
	}
}

func (c *checker) restrictedParam(expr syntax.NodeID) EntityID {
	return c.restrictedAliasOf(expr)
}

// restrictedAliasOf conservatively traces expr for a tainted non-escaping parameter. Record-literal
// fields are tainted separately as stores at construction (infer_record.go), not as a source here.
func (c *checker) restrictedAliasOf(expr syntax.NodeID) EntityID {
	switch c.t.Kind(expr) {
	case syntax.Paren:
		return c.restrictedAliasOf(syntax.NodeID(c.t.Nodes[expr].Lhs))
	case syntax.Block:
		return c.blockTailAlias(expr)
	case syntax.IfExpr:
		s := ifExpr(c.t, expr)
		if p := c.blockTailAlias(s.Then); p != 0 {
			return p
		}
		if s.Else == 0 {
			return 0
		}
		return c.restrictedAliasOf(s.Else)
	case syntax.IfLetExpr:
		s := ifLet(c.t, expr)
		if p := c.blockTailAlias(s.Then); p != 0 {
			return p
		}
		if s.Else == 0 {
			return 0
		}
		return c.restrictedAliasOf(s.Else)
	case syntax.MatchExpr:
		for _, arm := range c.t.Children(syntax.NodeID(c.t.Nodes[expr].Rhs)) {
			if p := c.restrictedAliasOf(matchArm(c.t, arm).Body); p != 0 {
				return p
			}
		}
		return 0
	case syntax.TupleLit:
		for _, elem := range c.t.Children(expr) {
			if p := c.restrictedAliasOf(elem); p != 0 {
				return p
			}
		}
		return 0
	case syntax.CallExpr, syntax.WsCallExpr:
		return c.taintedCall[expr]
	default:
		p := c.aliasChase(c.info.Uses[expr])
		if !c.restrictedEntity(p) {
			return 0
		}
		return p
	}
}

// blockTailAlias is restrictedAliasOf for a block's tail expression; 0 when the block has none.
func (c *checker) blockTailAlias(body syntax.NodeID) EntityID {
	stmts := c.t.Children(body)
	if len(stmts) == 0 || c.t.Kind(stmts[len(stmts)-1]) != syntax.ExprStmt {
		return 0
	}
	return c.restrictedAliasOf(syntax.NodeID(c.t.Nodes[stmts[len(stmts)-1]].Lhs))
}

// taintPatternBindings taints every name a pattern binds, not only the one structurally holding rp,
// since tuple/record patterns aren't tracked field-by-field.
func (c *checker) taintPatternBindings(p syntax.NodeID, rp EntityID) {
	if rp == 0 || p == 0 {
		return
	}
	if ent := c.info.Defs[p]; ent != 0 {
		c.escapeAlias[ent] = rp
	}
	c.t.EachChild(p, func(child syntax.NodeID) { c.taintPatternBindings(child, rp) })
}
