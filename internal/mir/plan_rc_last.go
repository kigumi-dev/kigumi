package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// A parameter is left out: a caller may hand one box to two parameters, and the callee's copy keeps them apart.
func (p *Program) movable(f *Func, o LocalID) bool {
	l := f.Locals[o]
	return !l.Cell && !l.Borrowed && p.copyable(l.Type) && p.R.CopyAllocates(l.Type) && !slices.Contains(f.Params, o)
}

func (w *shareFacts) ownsBox(o LocalID) bool {
	rs := w.rootsOf(o)
	return !rs.other && len(rs.refs) == 1 && rs.refs[0].l == o
}

func (p *Program) returnsLast(f *Func, a, o LocalID, w *shareFacts, at Point) bool {
	return p.movable(f, o) && w.ownsBox(o) && w.freshAlias(a, o, at)
}

func (p *Program) bindsLast(f *Func, in Inst, o LocalID, w *shareFacts, at Point) bool {
	return in.Op == OpShare && p.movable(f, o) && w.ownsBox(o) && !w.seenBut(o, in.Dst, at) && w.freshAlias(in.Args[0], o, at)
}

func (w *shareFacts) seenBut(l, dst LocalID, at Point) bool {
	lv := w.liveness()
	if lv.after(l, at.Block, at.Inst) {
		return true
	}
	for _, v := range w.viewersOf(l) {
		if v != dst && lv.after(v, at.Block, at.Inst) {
			return true
		}
	}
	return false
}

func (p *Program) retainsArg(f *Func, in Inst, i int, o LocalID, w *shareFacts) bool {
	g := p.ByEnt[in.Ent]
	if in.Op != OpCall || g == nil || g.Async || p.R.Fn(in.Ent).Abi != "" || p.R.Fn(in.Ent).Naked || p.R.Types.Kind(f.Locals[o].Type) == sem.KRef || f.Locals[o].Type == sem.TyBytes {
		return false
	}
	rs := w.rootsOf(o)
	if rs.other || w.heldAcrossWrite(in, i) {
		return false
	}
	return !slices.ContainsFunc(rs.refs, func(r rootRef) bool { return p.lentLocal(f, r.l) })
}
