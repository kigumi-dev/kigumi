package sem

import "kigumi/internal/syntax"

func (c *checker) notePatternAliases(pat, subject syntax.NodeID, matched bool) {
	if s := c.t.Unparen(subject); c.t.Kind(s) == syntax.MoveExpr {
		subject = c.t.View(s).AsMove().Operand().ID()
	}
	root := c.exprAlias(subject)
	if root == 0 && matched {
		if p, ok := c.placeUnder(subject); ok {
			root = p.ent
		}
	}
	if root != 0 {
		c.markPatternAliases(pat, root)
	}
}

func (c *checker) exprAlias(n syntax.NodeID) EntityID {
	if n == 0 {
		return 0
	}
	if t := c.info.Types[n]; t != 0 && c.r.isCopy(c.vars.resolve(t)) {
		return 0
	}
	switch c.t.Kind(n) {
	case syntax.Ident:
		return c.methodValues.aliases[c.info.Uses[n]]
	case syntax.BorrowExpr, syntax.Lambda:
		return 0
	case syntax.Block:
		if tail := c.t.View(n).AsBlock().Tail(); tail.Valid() {
			return c.exprAlias(tail.ID())
		}
		return 0
	case syntax.IfExpr:
		s := ifExpr(c.t, n)
		return c.firstAlias(s.Then, s.Else)
	case syntax.IfLetExpr:
		s := ifLet(c.t, n)
		return c.firstAlias(s.Then, s.Else)
	case syntax.MatchExpr:
		for arm := range c.t.View(n).AsMatch().Arms().IDs() {
			if root := c.exprAlias(matchArm(c.t, arm).Body); root != 0 {
				return root
			}
		}
		return 0
	}
	for child := range c.t.View(n).Children() {
		if root := c.exprAlias(child.ID()); root != 0 {
			return root
		}
	}
	return 0
}

func (c *checker) firstAlias(a, b syntax.NodeID) EntityID {
	if root := c.exprAlias(a); root != 0 || b == 0 {
		return root
	}
	return c.exprAlias(b)
}

func (c *checker) markPatternAliases(p syntax.NodeID, root EntityID) {
	if p == 0 {
		return
	}
	if ent := c.info.Defs[p]; ent != 0 && c.r.Entities[ent].Kind == EntLocal && ent != root {
		if c.methodValues.aliases == nil {
			c.methodValues.aliases = map[EntityID]EntityID{}
		}
		c.methodValues.aliases[ent] = root
	}
	for child := range c.t.View(p).Children() {
		c.markPatternAliases(child.ID(), root)
	}
}

func (c *checker) noteAssignAlias(n syntax.NodeID) {
	a := c.t.View(n).AsAssign()
	root := c.exprAlias(a.Value().ID())
	if root == 0 {
		return
	}
	for _, ent := range c.assignRoots(a.Target().ID(), nil) {
		if _, aliased := c.methodValues.aliases[ent]; aliased || ent == root {
			continue
		}
		if c.methodValues.aliases == nil {
			c.methodValues.aliases = map[EntityID]EntityID{}
		}
		c.methodValues.aliases[ent] = root
		c.recheckLoopFieldMoves(ent, root, n)
	}
}

func (c *checker) assignRoots(target syntax.NodeID, out []EntityID) []EntityID {
	if c.t.Kind(c.t.Unparen(target)) == syntax.TupleLit {
		for elem := range c.t.List(c.t.Unparen(target)).IDs() {
			out = c.assignRoots(elem, out)
		}
		return out
	}
	if ent := c.fieldRoot(target); ent != 0 {
		out = append(out, ent)
	}
	return out
}
