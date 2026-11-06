package sem

import "kigumi/internal/compiler/syntax"

// scalarBorrow is a `&mut` that may be scalar; MIR aliases a scalar `&mut` of a
// bare local to the local's cell, so its final type decides EfAddrTaken.
type scalarBorrow struct {
	inner      syntax.NodeID
	ref        TypeID
	arg, async bool
	// captured: the innermost lambda open at the borrow captured the borrowed local.
	captured bool
}

type refShape uint8

const (
	refOther refShape = iota
	refScalar
	refOpen
)

// mutRefShape is ScalarRefElem on a type still being inferred: an open
// literal variable can only settle to a number, any other one must wait.
func (c *checker) mutRefShape(t TypeID) refShape {
	t = c.vars.resolve(t)
	if _, ok := c.r.ScalarRefElem(t); ok {
		return refScalar
	}
	n := c.r.Types.Node(t)
	switch {
	case n.Kind != KRef || n.Flags&flagMut == 0 || c.r.Types.Kind(n.Elem) != KVar:
		return refOther
	case c.vars.pendingLiteral(n.Elem):
		return refScalar
	}
	return refOpen
}

func (c *checker) noteScalarBorrow(n, inner syntax.NodeID, refT TypeID) {
	b := scalarBorrow{inner: inner, ref: refT, arg: c.argPlace[n], async: c.argAsyncPlace[n], captured: c.capturedHere(c.rootOf(inner))}
	switch c.mutRefShape(refT) {
	case refScalar:
		c.markScalarBorrow(b, nil)
	case refOpen:
		c.openBorrows = append(c.openBorrows, b)
	}
}

// settleBorrows decides the borrows a boundary settled; the write itself was noted at
// the borrow, but sharing the cell with capturing closures waited for the scalar type.
func (c *checker) settleBorrows() {
	kept := c.openBorrows[:0]
	for _, b := range c.openBorrows {
		switch c.mutRefShape(b.ref) {
		case refOpen:
			kept = append(kept, b)
		case refScalar:
			c.markScalarBorrow(b, c.settledCaptureWrite)
		}
	}
	c.openBorrows = kept
}

func (c *checker) markScalarBorrow(b scalarBorrow, written func(scalarBorrow, EntityID)) {
	ent, ok := c.identPlaceEnt(b.inner)
	switch {
	case ok && c.r.Types.Kind(c.vars.resolve(c.r.Entities[ent].Type)) == KRef:
		// `.value` through a reference reaches the referent's own cell; the local is not the storage.
	case ok:
		c.r.Entities[ent].Flags |= EfAddrTaken
		if written != nil {
			written(b, ent)
		}
	case c.indexAssignUnsupported(b.inner) || !c.isMutablePlace(b.inner) || c.viewRefuses(b.inner, accessMutBorrow):
	case b.async:
		c.errAt(b.inner, cMutPlaceAsyncArg)
	case !b.arg:
		c.errAt(b.inner, cMutPlaceEscape)
	}
}

func (c *checker) settledCaptureWrite(b scalarBorrow, ent EntityID) {
	if b.captured {
		c.promoteCaptures(b.inner, ent, false)
		return
	}
	c.shareOutsideWrite(b.inner, ent)
}
