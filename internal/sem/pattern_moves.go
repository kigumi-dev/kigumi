package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type patMoveLog struct {
	path  []EntityID
	moved [][]EntityID
}

func (c *checker) notePatternMove(t TypeID) {
	if !c.patBorrow && c.ownsType(t) {
		c.patMoved.moved = append(c.patMoved.moved, slices.Clone(c.patMoved.path))
	}
}

func (c *checker) patternInField(fld EntityID, f func()) {
	if fld == 0 {
		f()
		return
	}
	c.patMoved.path = append(c.patMoved.path, fld)
	f()
	c.patMoved.path = c.patMoved.path[:len(c.patMoved.path)-1]
}

func (c *checker) reservePatternMoves(scrutinee syntax.NodeID, moved [][]EntityID) {
	if len(moved) == 0 {
		return
	}
	p, ok := c.placeUnder(scrutinee)
	if !ok || !c.ownsValue(p.ent) {
		return
	}
	for _, path := range moved {
		c.reservePlace(scrutinee, reservedPlace{ent: p.ent, fields: slices.Concat(p.fields, path)})
	}
}
