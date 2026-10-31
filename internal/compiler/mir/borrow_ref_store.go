package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A reference read out of a stored value points somewhere else and is not followed.
func (c *borrowCheck) refRoot(l midir.LocalID) (midir.LocalID, bool) {
	tt := c.p.R.Types
	isRef := func(l midir.LocalID) bool { return tt.Kind(c.f.Locals[l].Type) == sem.KRef }
	for range 64 {
		if slices.Contains(c.f.Params, l) {
			return l, true
		}
		var in *midir.Inst
		if c.f.Locals[l].Ent != 0 {
			if !isRef(l) {
				return l, true
			}
			in = c.soleDef(l)
		} else {
			in = c.definition(l)
		}
		if in == nil || len(in.Args) == 0 {
			return 0, false
		}
		switch in.Op {
		case midir.OpBorrow, midir.OpCopy, midir.OpMove, midir.OpAlias, midir.OpShare, midir.OpNewCell:
		case midir.OpPayload, midir.OpField, midir.OpIndex, midir.OpUnbox:
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
