package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Every element access is one step, so two element paths always overlap.
type step struct {
	op  midir.Opcode
	idx int
}

type wpath struct {
	path  []step
	whole bool
}

type rootAt struct {
	l  midir.LocalID
	at Point
}

// A reference read out of a field points at storage elsewhere, so the walk stops there.
func (w *shareFacts) placeOf(v midir.LocalID) (midir.LocalID, []step, bool) {
	var rev []step
	for range w.nLocals {
		if w.f.Locals[v].Ent != 0 {
			slices.Reverse(rev)
			return v, rev, true
		}
		ds := w.defs[v]
		if len(ds) != 1 || len(ds[0].Args) == 0 {
			return 0, nil, false
		}
		switch in := *ds[0]; {
		case projects(in):
			if w.p.R.Types.Kind(w.f.Locals[v].Type) == sem.KRef {
				return 0, nil, false
			}
			rev = append(rev, stepOf(in))
		case in.Op == midir.OpShare && in.Copy, in.Op == midir.OpCopy && in.Share, !passesOn(in.Op):
			return 0, nil, false
		}
		v = ds[0].Args[0]
	}
	return 0, nil, false
}

func stepOf(in midir.Inst) step {
	switch in.Op {
	case midir.OpField, midir.OpPayload:
		return step{in.Op, in.Index}
	case midir.OpFieldMove:
		return step{midir.OpField, in.Index}
	case midir.OpBuiltin:
		return step{op: midir.OpIndex}
	}
	return step{op: in.Op}
}

func overlaps(s []step, p wpath) bool {
	n := min(len(s), len(p.path))
	return slices.Equal(s[:n], p.path[:n]) && (p.whole || len(s) <= len(p.path))
}

// A root bound again after at may hold other boxes, so its paths no longer compare.
func (w *shareFacts) exactRoot(src midir.LocalID, r midir.LocalID, at Point) ([]step, bool) {
	l, s, ok := w.placeOf(src)
	if !ok || l != r || w.reachable(at, w.sets[r]) {
		return nil, false
	}
	return s, true
}

func (w *shareFacts) touches(r midir.LocalID, x Point, s []step) bool {
	for _, p := range w.paths[rootAt{r, x}] {
		if overlaps(s, p) {
			return true
		}
	}
	return false
}
