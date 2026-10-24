package sem

import "kigumi/internal/syntax"

type copyCarry uint8

const (
	carryNone copyCarry = iota
	carryDirect
	carryInside
)

// copyCarrier finds the `&mut` match copy binding whose `&mut` n's value is (carryDirect) or holds in a tuple (carryInside).
func (c *checker) copyCarrier(n syntax.NodeID) (EntityID, copyCarry) {
	n = c.t.Unparen(n)
	if src, ok := c.held.copyLit[n]; ok {
		return src, carryInside
	}
	if ent, ok := c.identPlaceEnt(n); ok {
		if _, ok := c.held.scalarCopy[ent]; ok {
			return ent, carryDirect
		}
		if src, ok := c.held.copyHold[ent]; ok {
			return src, carryInside
		}
		return 0, carryNone
	}
	if c.t.Kind(n) != syntax.MemberExpr {
		return 0, carryNone
	}
	src, k := c.copyCarrier(c.t.View(n).AsMember().Base().ID())
	if k != carryInside {
		return 0, carryNone
	}
	t := c.vars.resolve(c.info.Types[n])
	if _, ok := c.r.ScalarRefElem(t); ok {
		return src, carryDirect
	}
	if c.holdsMutRef(t, 0) {
		return src, carryInside
	}
	return 0, carryNone
}

func (c *checker) holdsMutRef(t TypeID, depth int) bool {
	t = c.vars.resolve(t)
	if c.r.Types.RefMut(t) {
		return true
	}
	for _, a := range c.r.Types.Node(t).Args {
		if depth < 8 && c.holdsMutRef(a, depth+1) {
			return true
		}
	}
	return false
}

// noteCopyLit marks a tuple built from a copy's `&mut`: its uses, not the literal, decide whether a write is lost.
func (c *checker) noteCopyLit(n, item syntax.NodeID) {
	if len(c.held.scalarCopy) == 0 {
		return
	}
	for _, leaf := range c.tailLeaves(item, 0) {
		if src, k := c.copyCarrier(leaf); k != carryNone {
			if c.held.copyLit == nil {
				c.held.copyLit = map[syntax.NodeID]EntityID{}
			}
			c.held.copyLit[c.t.Unparen(n)] = src
			return
		}
	}
}

func (c *checker) inheritScalarCopy(ent EntityID, init syntax.NodeID) {
	if ent == 0 || len(c.held.scalarCopy) == 0 {
		return
	}
	for _, leaf := range c.tailLeaves(init, 0) {
		if src, k := c.copyCarrier(leaf); k != carryNone {
			c.bindCopy(ent, src)
			return
		}
	}
}

func (c *checker) inheritPatternCopy(expr syntax.NodeID, bound []EntityID) {
	if len(c.held.scalarCopy) == 0 {
		return
	}
	for _, leaf := range c.tailLeaves(expr, 0) {
		if src, k := c.copyCarrier(leaf); k == carryInside {
			for _, ent := range bound {
				c.bindCopy(ent, src)
			}
			return
		}
	}
}

func (c *checker) bindCopy(ent, src EntityID) {
	t := c.vars.resolve(c.r.Entities[ent].Type)
	if _, ok := c.r.ScalarRefElem(t); ok {
		c.held.scalarCopy[ent] = c.held.scalarCopy[src]
		return
	}
	if c.holdsMutRef(t, 0) {
		if c.held.copyHold == nil {
			c.held.copyHold = map[EntityID]EntityID{}
		}
		c.held.copyHold[ent] = src
	}
}

func (c *checker) storeScalarCopy(lhs, rhs syntax.NodeID, target TypeID) {
	ent, ok := c.identPlaceEnt(lhs)
	if !ok || c.t.Kind(c.t.Unparen(lhs)) != syntax.Ident {
		c.rejectScalarCopyStore(rhs, target)
		return
	}
	if len(c.held.scalarCopy) == 0 || !c.holdsMutRef(c.r.Entities[ent].Type, 0) || c.r.Types.RefMut(c.vars.resolve(c.r.Entities[ent].Type)) {
		return
	}
	for _, leaf := range c.tailLeaves(rhs, 0) {
		if src, k := c.copyCarrier(leaf); k == carryInside {
			c.bindCopy(ent, src)
			return
		}
	}
}
