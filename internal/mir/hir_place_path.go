package mir

import (
	"slices"

	"kigumi/internal/hir"
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

func (w localWrites) hitsPlace(pl *hir.Place) bool {
	if !rootedAt(pl.Base, w.ent) {
		return false
	}
	path := placePath(pl.Base)
	if pl.Kind == hir.PlaceField {
		path = append(path, pl.Index)
	}
	return fieldPathsOverlap(path, w.path)
}
