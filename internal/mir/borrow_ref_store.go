package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// A reference read out of a stored value points somewhere else and is not followed.
func (c *borrowCheck) refRoot(l LocalID) (LocalID, bool) {
	tt := c.p.R.Types
	isRef := func(l LocalID) bool { return tt.Kind(c.f.Locals[l].Type) == sem.KRef }
	for range 64 {
		if slices.Contains(c.f.Params, l) {
			return l, true
		}
		var in *Inst
		if c.f.Locals[l].Ent != 0 {
			if !isRef(l) {
				return l, true
			}
			in = soleDef(c.f, l)
		} else {
			in = c.definition(l)
		}
		if in == nil || len(in.Args) == 0 {
			return 0, false
		}
		switch in.Op {
		case OpBorrow, OpCopy, OpMove, OpAlias, OpShare, OpNewCell:
		case OpPayload, OpField, OpIndex, OpUnbox:
			if isRef(l) && !isRef(in.Args[0]) {
				return 0, false
			}
		default:
			return 0, false
		}
		l = in.Args[0]
	}
	return 0, false
}
