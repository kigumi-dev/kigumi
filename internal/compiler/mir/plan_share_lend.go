package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type calleeRef struct {
	g *midir.Func
	i int
}

// A lending is a write only if a callee writes through it, known once the summaries settle.
type lentSite struct {
	at   Point
	base midir.LocalID
	to   []calleeRef
}

func (w *shareFacts) calleeParam(in midir.Inst, i int) ([]calleeRef, bool) {
	g := w.p.ByEnt[in.Ent]
	if in.Op != midir.OpCall || g == nil || i >= len(g.Params) || w.p.R.Types.Kind(g.Locals[g.Params[i]].Type) != sem.KRef {
		return nil, false
	}
	return []calleeRef{{g, i}}, true
}

func (w *shareFacts) lentTo(d midir.LocalID) ([]calleeRef, bool) {
	var out []calleeRef
	for _, u := range w.uses[d] {
		if u.term {
			return nil, false
		}
		in := w.inst(u.at)
		if in.Op == midir.OpDrop || in.Op == midir.OpRelease {
			continue
		}
		to, ok := w.calleeParam(in, u.i)
		if !ok {
			return nil, false
		}
		out = append(out, to...)
	}
	return out, true
}

func (w *shareFacts) lendOrWrite(base midir.LocalID, at Point, to []calleeRef, lent bool) {
	if !lent {
		w.noteWrite(base, at, true)
		return
	}
	w.lent = append(w.lent, lentSite{at, base, to})
}

func (s *shareSummary) writesThrough(to []calleeRef) bool {
	for _, c := range to {
		if m, ok := s.mutates[c.g]; !ok || m[c.i] {
			return true
		}
	}
	return false
}

func (w *shareFacts) noteMutates() bool {
	f, changed := w.f, false
	for j, prm := range f.Params {
		if !w.sum.mutates[f][j] && w.p.R.Types.Kind(f.Locals[prm].Type) == sem.KRef && w.mutatesParam(prm) {
			w.sum.mutates[f][j], changed = true, true
		}
	}
	return changed
}

func (w *shareFacts) mutatesParam(prm midir.LocalID) bool {
	if len(w.own[prm])+len(w.inner[prm])+len(w.blind) > 0 || w.escapes(prm) {
		return true
	}
	for _, ls := range w.lent {
		if r, _, ok := w.placeOf(ls.base); (!ok || r == prm) && w.sum.writesThrough(ls.to) {
			return true
		}
	}
	return false
}

// A reference that leaves the call can be written through later, outside this body's writes.
func (w *shareFacts) escapes(prm midir.LocalID) bool {
	for _, u := range w.uses[prm] {
		if u.term {
			return true
		}
		switch in := w.inst(u.at); {
		case projects(in) && u.i == 0 && !w.mutRef(in.Dst), (in.Op == midir.OpSetField || in.Op == midir.OpSetIndex) && u.i == 0:
		case in.Op == midir.OpBorrow, in.Op == midir.OpCall, in.Op == midir.OpDrop, in.Op == midir.OpRelease, in.Op == midir.OpBuiltin && in.Str == "iter.len":
		default:
			return true
		}
	}
	return false
}

func (w *shareFacts) mutRef(v midir.LocalID) bool {
	_, mut := w.p.R.MutRefElem(w.f.Locals[v].Type)
	return mut
}

func (w *shareFacts) settleLent() {
	for _, ls := range w.lent {
		if w.sum.writesThrough(ls.to) {
			w.noteWrite(ls.base, ls.at, true)
		}
	}
}
