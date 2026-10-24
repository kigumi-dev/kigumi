package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

func (c *checker) noteMutFieldValue(n syntax.NodeID, t TypeID) {
	c.rejectSharedMutRef(n, t)
	c.noteFieldMove(n, t)
}

// Reading through a reference takes no `&mut` out of its place.
func (c *checker) synthRead(n syntax.NodeID) TypeID {
	c.held.wantRead = true
	return c.synth(n)
}

// A `&mut T` value is move-only, so reading the field by value moves it.
func (c *checker) noteFieldMove(n syntax.NodeID, t TypeID) {
	if c.t.Kind(n) != syntax.MemberExpr || c.held.reborrows[n] || !c.r.Types.RefMut(c.vars.resolve(t)) {
		return
	}
	if fld := c.info.Uses[n]; fld == 0 || c.r.Entities[fld].Kind != EntField || c.behindShared(n) || c.indexRooted(n) {
		return
	}
	baseType := c.readThrough(c.vars.resolve(c.info.Types[c.t.Unparen(c.t.View(n).AsMember().Base().ID())]))
	if bn := c.r.Types.Node(baseType); bn.Kind == KNamed && c.isSharedEnt(bn.Ent) {
		c.errAt(n, cMoveShared, baseType)
		return
	}
	if !c.checkMoveOut(n, t, baseType, true) {
		return
	}
	if c.info.FieldMoves == nil {
		c.info.FieldMoves = map[syntax.NodeID]bool{}
	}
	c.info.FieldMoves[n] = true
}

// As with a bare `&mut` binding, a declared `&mut T` parameter of a synchronous call reborrows instead of moving.
func (c *checker) markReborrowArg(a syntax.NodeID, param TypeID, decl EntityID, async bool) {
	a = c.t.Unparen(a)
	if async || c.t.Kind(a) != syntax.MemberExpr {
		return
	}
	if decl != 0 {
		param = c.r.Entities[decl].Type
	}
	if !c.r.Types.RefMut(c.vars.resolve(param)) {
		return
	}
	if c.held.reborrows == nil {
		c.held.reborrows = map[syntax.NodeID]bool{}
	}
	c.held.reborrows[a] = true
}

func (c *checker) rejectTwiceLent(args []syntax.NodeID) {
	for i, a := range args {
		p, ok := c.lentField(a)
		if !ok {
			continue
		}
		for j, b := range args {
			if j == i || c.copiedBeforeCall(b) {
				continue
			}
			if q, ok := c.lentArgPlace(b); ok && q.Root == p.Root && pathsOverlap(p.Fields, q.Fields) {
				c.errAt(c.t.Unparen(a), cMutFieldLentTwice, c.placeName(a))
				return
			}
		}
	}
}

// A Copy value without references is read before the call, so the callee keeps no path to its place.
func (c *checker) copiedBeforeCall(a syntax.NodeID) bool {
	a = c.t.Unparen(a)
	if c.t.Kind(a) == syntax.BorrowExpr {
		return false
	}
	t := c.vars.resolve(c.info.Types[a])
	return c.r.isCopy(t) && !c.carriesRef(t, 0)
}

func (c *checker) carriesRef(t TypeID, depth int) bool {
	t = c.vars.resolve(t)
	n := c.r.Types.Node(t)
	switch n.Kind {
	case KRef, KPtr, KFn, KClosure, KParam, KVar:
		return true
	}
	if depth >= 8 {
		return true
	}
	if slices.ContainsFunc(n.Args, func(a TypeID) bool { return c.carriesRef(a, depth+1) }) {
		return true
	}
	if n.Kind != KNamed || c.r.Entities[n.Ent].Kind != EntType {
		return false
	}
	return slices.ContainsFunc(c.r.typeDecl(n.Ent).Fields, func(f EntityID) bool { return c.carriesRef(c.r.Entities[f].Type, depth+1) })
}

func (c *checker) rejectLentRecv(base syntax.NodeID, args []syntax.NodeID) {
	q, ok := c.lentArgPlace(base)
	if !ok {
		return
	}
	for _, a := range args {
		a = c.t.View(a).ArgValue().ID()
		if p, ok := c.lentField(a); ok && q.Root == p.Root && pathsOverlap(p.Fields, q.Fields) {
			c.errAt(c.t.Unparen(a), cMutFieldLentTwice, c.placeName(a))
			return
		}
	}
}

func (c *checker) lentField(a syntax.NodeID) (Place, bool) {
	a = c.t.Unparen(a)
	if !c.held.reborrows[a] || !c.r.Types.RefMut(c.vars.resolve(c.info.Types[a])) {
		return Place{}, false
	}
	return c.buildPlace(a)
}

func (c *checker) lentArgPlace(a syntax.NodeID) (Place, bool) {
	a = c.t.Unparen(a)
	if c.t.Kind(a) == syntax.BorrowExpr {
		a = c.t.View(a).AsBorrow().Operand().ID()
	}
	return c.buildPlace(a)
}

func pathsOverlap(a, b []EntityID) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
