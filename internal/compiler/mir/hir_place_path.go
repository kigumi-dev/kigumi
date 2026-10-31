package mir

import (
	"slices"

	"kigumi/internal/compiler/hir"
)

// placePath is the field path from e's root local; an index drops what lies past it, since the element is not known.
func placePath(e *hir.Expr) []int {
	var rev []int
	for e != nil && e.Kind != hir.Local {
		switch e.Kind {
		case hir.Field, hir.FieldMove:
			rev = append(rev, e.Index)
		case hir.Index:
			rev = rev[:0]
		case hir.Wrap, hir.DefinedWrap, hir.DefinedUnwrap:
		default:
			return nil
		}
		e = e.Args[0]
	}
	slices.Reverse(rev)
	return rev
}

// placeSteps is e's full field path from its root local, with -1 for an element, whose index is not known.
func placeSteps(e *hir.Expr) []int {
	var rev []int
	for e != nil && e.Kind != hir.Local {
		switch e.Kind {
		case hir.Field, hir.FieldMove:
			rev = append(rev, e.Index)
		case hir.Index:
			rev = append(rev, -1)
		case hir.Wrap, hir.DefinedWrap, hir.DefinedUnwrap:
		default:
			return nil
		}
		e = e.Args[0]
	}
	slices.Reverse(rev)
	return rev
}

func (w localWrites) hits(e *hir.Expr) bool {
	return rootedAt(e, w.ent) && w.covers(placePath(e), placeSteps(e))
}

func (w localWrites) hitsPlace(pl *hir.Place) bool {
	if !rootedAt(pl.Base, w.ent) {
		return false
	}
	path, steps := placePath(pl.Base), placeSteps(pl.Base)
	if pl.Kind == hir.PlaceField {
		path, steps = append(path, pl.Index), append(steps, pl.Index)
	} else {
		steps = append(steps, -1)
	}
	return w.covers(path, steps)
}

// covers reports a place overlapping the receiver's path, or with whole only one whose steps lead to the receiver,
// any element standing for any other; a deeper place lies inside the receiver or beside it, where the lent view sees the write.
func (w localWrites) covers(path, steps []int) bool {
	if w.whole {
		return len(steps) <= len(w.steps) && slices.Equal(steps, w.steps[:len(steps)])
	}
	return fieldPathsOverlap(path, w.path)
}

// above reports whether place e strictly contains the receiver's place, so a `mut self` call on it may replace the receiver.
func (w localWrites) above(e *hir.Expr) bool {
	return len(placeSteps(e)) < len(w.steps)
}

// movesOut reports a move of place e that leaves the receiver without its value.
func (w localWrites) movesOut(e *hir.Expr) bool {
	if w.whole && w.deep {
		return rootedAt(e, w.ent) && fieldPathsOverlap(placeSteps(e), w.steps)
	}
	return w.hits(e)
}
