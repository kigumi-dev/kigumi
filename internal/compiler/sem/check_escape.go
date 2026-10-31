package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) restrictedEntity(ent EntityID) bool {
	ent = c.aliasChase(ent)
	return ent != 0 && c.r.Entities[ent].Kind == EntParam && c.ownParam(c.r.Entities[ent].Parent) && c.r.nonEscapingParam(ent)
}

func (c *checker) ownParam(owner EntityID) bool {
	if owner == c.fn {
		return true
	}
	for _, ctx := range c.lambdas {
		if ctx.ent == owner {
			return true
		}
	}
	return false
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
		return c.restrictedAliasOf(c.t.View(expr).AsParen().Inner().ID())
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
		for arm := range c.t.View(expr).AsMatch().Arms().IDs() {
			if p := c.restrictedAliasOf(matchArm(c.t, arm).Body); p != 0 {
				return p
			}
		}
		return 0
	case syntax.TupleLit:
		for elem := range c.t.List(expr).IDs() {
			if p := c.restrictedAliasOf(elem); p != 0 {
				return p
			}
		}
		return 0
	case syntax.CallExpr, syntax.WsCallExpr:
		return c.taintedCall[expr]
	case syntax.MemberExpr, syntax.OptMemberExpr, syntax.BracketExpr, syntax.Unary, syntax.BorrowExpr, syntax.MoveExpr, syntax.TryExpr:
		return c.projectionAlias(expr)
	default:
		p := c.aliasChase(c.info.Uses[expr])
		if !c.restrictedEntity(p) || !c.holdsFn(expr, p) {
			return 0
		}
		return p
	}
}

// blockTailAlias is restrictedAliasOf for a block's tail expression; 0 when the block has none.
func (c *checker) blockTailAlias(body syntax.NodeID) EntityID {
	tail := c.t.View(body).AsBlock().Tail()
	if !tail.Valid() {
		return 0
	}
	return c.restrictedAliasOf(tail.ID())
}

// taintPatternBindings taints every name a pattern binds, not only the one structurally holding rp,
// since tuple/record patterns aren't tracked field-by-field.
func (c *checker) taintPatternBindings(p syntax.NodeID, rp EntityID) {
	if rp == 0 || p == 0 {
		return
	}
	if ent := c.info.Defs[p]; ent != 0 && c.r.mayContainFn(c.vars.resolve(c.r.Entities[ent].Type), map[TypeID]bool{}) {
		c.escapeAlias[ent] = rp
	}
	for child := range c.t.View(p).Children() {
		c.taintPatternBindings(child.ID(), rp)
	}
}
