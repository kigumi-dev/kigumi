package sem

import "kigumi/internal/compiler/syntax"

// An owned field moves implicitly only as the consuming operand itself (argument, receiver, scrutinee,
// payload, await, result); bound by `let`, built into an aggregate or picked by a join it needs `move`.
func (c *checker) rejectFieldValue(n syntax.NodeID) {
	for _, leaf := range c.valueLeaves(n) {
		c.rejectFieldLeaf(leaf, takeConsumed)
	}
}

// rejectBorrowedFieldResult: a result may take an owned parameter's field, never a borrowed one's.
func (c *checker) rejectBorrowedFieldResult(leaves []syntax.NodeID) {
	for _, leaf := range leaves {
		for _, v := range c.valueLeaves(leaf) {
			c.rejectFieldLeaf(v, takeReturned)
		}
	}
}

func (c *checker) rejectFieldLeaf(n syntax.NodeID, take fieldTake) bool {
	if owner := c.chainDropOwner(n); owner != 0 && dropOwnerChecks[take][pathTemp] && c.ownsType(c.vars.resolve(c.info.Types[n])) {
		c.rejectDropOwnerTake(n, owner)
		return true
	}
	if !c.fieldMember(n) {
		return false
	}
	k := c.r.Entities[c.info.Uses[n]].Kind
	t := c.vars.resolve(c.info.Types[n])
	if t == 0 || t == TyPoison || c.r.isCopy(t) || c.r.Types.Kind(t) == KRef || c.indexRooted(n) || c.r.hasErrorAt(c.f, n) {
		return false
	}
	mode := c.pathMode(n)
	switch {
	case k == EntType && (mode == pathOwned || mode == pathTemp):
		// `.value` retypes its base's storage, so the base is the place read.
		return c.rejectFieldLeaf(c.t.Unparen(c.t.View(n).AsMember().Base().ID()), take)
	case c.rejectTake(n, t, take):
		return true
	case fieldTakeRejects[take][mode]:
		name := c.placeName(n)
		c.errAtHelp(n, c.noCloneHelp(t, "write `move "+name+"` to take it out, or borrow it with `&"+name+"`"), cFieldValueMove, name, t, name, name)
		return true
	}
	return false
}

// rejectBranchFieldArg: a plain `use(w.a)` stays an implicit move, but a field picked by an if / match / block tail or a fallback is not
// tracked as one.
func (c *checker) rejectBranchFieldArg(a syntax.NodeID) {
	u := c.t.Unparen(a)
	switch c.t.Kind(u) {
	case syntax.IfExpr, syntax.MatchExpr, syntax.Block:
		c.rejectFieldValue(a)
	default:
		if _, ok := c.fallbackMiss(u); ok {
			c.rejectFieldValue(a)
		}
	}
}

// valueLeaves are tailLeaves with each fallback opened up to the operand it yields on a miss.
func (c *checker) valueLeaves(n syntax.NodeID) []syntax.NodeID {
	var out []syntax.NodeID
	for _, leaf := range c.tailLeaves(n) {
		if miss, ok := c.fallbackMiss(leaf); ok {
			out = append(out, c.valueLeaves(miss)...)
			continue
		}
		out = append(out, c.t.Unparen(leaf))
	}
	return out
}

func (c *checker) fallbackMiss(n syntax.NodeID) (syntax.NodeID, bool) {
	if c.t.Kind(n) != syntax.Binary || c.info.Calls[n].Kind != CallFallback {
		return 0, false
	}
	rhs := c.t.View(n).AsBinary().Right().ID()
	if c.t.Kind(rhs) == syntax.Lambda {
		return c.t.View(rhs).AsLambda().Body().ID(), true
	}
	return rhs, true
}
