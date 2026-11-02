package sem

import "kigumi/internal/compiler/syntax"

// placeView is where a place sits under a slice link: the slice array itself, one of its slots, or
// inside an element object the slice shares with its source.
type placeView uint8

const (
	viewNone placeView = iota
	viewArray
	viewSlot
	viewElem
)

type placeAccess uint8

const (
	accessAssign placeAccess = iota
	accessMutBorrow
	accessMutSelf
)

// viewAllows: the slice's own storage is not the source's, so it is read-only; inside a shared element
// every access reaches the source's own object.
var viewAllows = [...][3]bool{
	viewNone:  {accessAssign: true, accessMutBorrow: true, accessMutSelf: true},
	viewArray: {},
	viewSlot:  {},
	viewElem:  {accessAssign: true, accessMutBorrow: true, accessMutSelf: true},
}

var accessWhat = [...]string{
	accessAssign:    "assigned",
	accessMutBorrow: "borrowed with `&mut`",
	accessMutSelf:   "the receiver of a `mut self` method",
}

// viewOf walks n's fields and indexes leaf to root and reports the deepest slice link it crosses.
func (c *checker) viewOf(n syntax.NodeID) (syntax.NodeID, placeView) {
	n = c.t.Unparen(n)
	switch c.t.Kind(n) {
	case syntax.BracketExpr:
		if c.bracketKind(n) == bracketSlice {
			return n, viewArray
		}
		slice, v := c.viewOf(c.t.View(n).AsBracket().Base().ID())
		switch v {
		case viewArray:
			return slice, viewSlot
		case viewSlot:
			return slice, viewElem
		}
		return slice, v
	case syntax.MemberExpr:
		if slice, v := c.viewOf(c.t.View(n).AsMember().Base().ID()); v != viewNone {
			return slice, viewElem
		}
	}
	return 0, viewNone
}

func (c *checker) rejectViewAccess(n syntax.NodeID, access placeAccess) bool {
	slice, v := c.viewOf(n)
	if viewAllows[v][access] {
		return false
	}
	where := "a slice"
	if v != viewArray {
		where = "in the slice `" + c.takeName(slice) + "`"
	}
	source := c.takeName(c.sliceSource(slice))
	c.errAt(n, cSliceReadOnly, c.takeName(n), where, source, accessWhat[access], source)
	return true
}

func (c *checker) viewRefuses(n syntax.NodeID, access placeAccess) bool {
	_, v := c.viewOf(n)
	return !viewAllows[v][access]
}

// sliceSource is the array a slice, or a slice of a slice, shares its elements with.
func (c *checker) sliceSource(slice syntax.NodeID) syntax.NodeID {
	src := c.t.Unparen(c.t.View(slice).AsBracket().Base().ID())
	for c.t.Kind(src) == syntax.BracketExpr && c.bracketKind(src) == bracketSlice {
		src = c.t.Unparen(c.t.View(src).AsBracket().Base().ID())
	}
	return src
}

// takeAtAllowed: n reads an element whose array may be the receiver of `.takeAt`, a `mut self` method.
func (c *checker) takeAtAllowed(n syntax.NodeID) bool {
	b, ok := c.indexRoot(n)
	return !ok || c.bracketKind(b) != bracketSlice && !c.viewRefuses(c.t.View(b).AsBracket().Base().ID(), accessMutSelf)
}

func (c *checker) indexValueHelp(n syntax.NodeID, t TypeID) string {
	if c.takeAtAllowed(n) {
		return c.noCloneHelp(t, "borrow it with `&`, or take it with `.takeAt`")
	}
	if h := c.noCloneHelp(t, "borrow it with `&`"); h != "" {
		return h
	}
	return "borrow it with `&`, or clone it"
}

func (c *checker) noteSliceView(local EntityID, init syntax.NodeID, borrowed bool) {
	if v := c.passThrough(init); borrowed && local != 0 && c.t.Kind(v) == syntax.BracketExpr && c.bracketKind(v) == bracketSlice {
		if c.sliceViews == nil {
			c.sliceViews = map[EntityID]syntax.NodeID{}
		}
		c.sliceViews[local] = v
	}
}
