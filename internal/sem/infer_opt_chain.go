package sem

import "kigumi/internal/syntax"

// OptChain is `a?.m(x).n`: one map over the postfix chain after `?.`. Body is
// the chain rebuilt on Local, which stands for the payload of Base.
type OptChain struct {
	Base  syntax.NodeID
	Body  syntax.NodeID
	Local EntityID
	Elem  TypeID
	// ByRef is set when Base is a place of a non-Copy payload: Local is then `&Elem` and Base stays in place.
	ByRef bool
}

// optChainRoot is the `?.` node a postfix chain rooted at n continues. The chain
// ends at the first node that is not a member, call or index, and at the next `?.`.
func (c *checker) optChainRoot(n syntax.NodeID) syntax.NodeID {
	switch c.t.Kind(n) {
	case syntax.MemberExpr, syntax.CallExpr, syntax.BracketExpr:
	default:
		return 0
	}
	for cur := n; ; {
		switch c.t.Kind(cur) {
		case syntax.OptMemberExpr:
			return cur
		case syntax.MemberExpr, syntax.BracketExpr:
			cur = c.t.View(cur).Base().ID()
		case syntax.CallExpr:
			cur = c.t.View(cur).AsCallLike().Callee().ID()
		default:
			return 0
		}
	}
}

func (c *checker) synthOptChain(n, o syntax.NodeID) TypeID {
	baseNode := c.t.View(o).AsOptMember().Base().ID()
	base := c.vars.resolve(c.synthPlace(baseNode))
	elem, ok := c.r.Types.IsOption(base)
	if !ok {
		if base != TyPoison {
			c.errAt(o, cOptMemberNotOption, base)
		}
		return TyPoison
	}
	byRef := c.isPlace(baseNode) && !c.r.isCopy(elem)
	holeType := elem
	if byRef {
		holeType = c.r.Types.Ref(elem, false)
	}
	hole := c.t.AddIdentAt(c.t.View(baseNode).Tok().Index())
	body := c.t.OptMemberAsMember(o, hole)
	spine := []syntax.NodeID{}
	for cur := n; cur != o; {
		spine = append(spine, cur)
		if c.t.Kind(cur) == syntax.CallExpr {
			cur = c.t.View(cur).AsCallLike().Callee().ID()
		} else {
			cur = c.t.View(cur).Base().ID()
		}
	}
	clones := make([]syntax.NodeID, len(spine)+1)
	clones[len(spine)] = body
	for i := len(spine) - 1; i >= 0; i-- {
		body = c.t.Rebased(spine[i], body)
		clones[i] = body
	}
	c.growTables()
	saved := c.scope
	c.pushScope(ScopeArm, n)
	local := c.declareLocal(hole, "?", holeType, false)
	if c.info.ChainHoles == nil {
		c.info.ChainHoles = map[syntax.NodeID]EntityID{}
	}
	c.info.ChainHoles[hole] = local
	t := c.vars.resolve(c.synth(body))
	c.popScope(saved)
	for i, orig := range append(spine, o) {
		c.info.Uses[orig] = c.info.Uses[clones[i]]
	}
	if t == TyPoison {
		return TyPoison
	}
	if byRef && c.rejectChainFieldLeaf(n, baseNode, body, t) {
		return TyPoison
	}
	if c.info.OptChains == nil {
		c.info.OptChains = map[syntax.NodeID]*OptChain{}
	}
	c.info.OptChains[n] = &OptChain{Base: baseNode, Body: body, Local: local, Elem: elem, ByRef: byRef}
	c.buildEdge(n, c.r.Types.Option(t))
	return c.r.Types.Option(t)
}

// rootIsChainHole reports whether n is the payload of a `?.` chain or a field path from it.
func (c *checker) rootIsChainHole(n syntax.NodeID) bool {
	for cur := n; len(c.info.ChainHoles) > 0; {
		switch c.t.Kind(cur) {
		case syntax.Paren, syntax.MemberExpr, syntax.BracketExpr:
			cur = c.t.View(cur).Base().ID()
		case syntax.Ident:
			_, ok := c.info.ChainHoles[cur]
			return ok
		default:
			return false
		}
	}
	return false
}

// rejectChainFieldLeaf: a chain that ends in a non-Copy field of a borrowed payload would hand out what the base still owns.
func (c *checker) rejectChainFieldLeaf(n, baseNode, body syntax.NodeID, t TypeID) bool {
	leaf := c.t.Unparen(body)
	if c.t.Kind(leaf) != syntax.MemberExpr {
		return false
	}
	return c.rejectOptFieldValue(n, baseNode, c.t.View(leaf).Tok().Text(), t)
}

func (c *checker) rejectOptFieldValue(n, baseNode syntax.NodeID, field string, t TypeID) bool {
	if c.indexRooted(baseNode) || c.info.Uses[n] == 0 || c.r.Entities[c.info.Uses[n]].Kind != EntField || c.r.isCopy(t) {
		return false
	}
	name := c.placeName(baseNode) + "?." + field
	c.errAtHelp(n, "match on the option and `move` the field out of the payload, or call a method that returns what you need", cFieldValueMove, name, t, name, name)
	return true
}
