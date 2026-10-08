package mir

import (
	"slices"

	"kigumi/internal/sem"
)

type shareSummary struct {
	p       *Program
	valued  map[sem.EntityID]bool
	retView map[*Func]bool
	taken   map[*Func][]bool
	mutates map[*Func][]bool
	facts   map[*Func]*shareFacts
	sealed  map[sem.TypeID]bool
}

func (p *Program) planShares(fs []*Func) *shareSummary {
	s := &shareSummary{p: p, valued: map[sem.EntityID]bool{}, retView: map[*Func]bool{}, taken: map[*Func][]bool{},
		mutates: map[*Func][]bool{}, facts: map[*Func]*shareFacts{}, sealed: map[sem.TypeID]bool{}}
	callers := map[*Func][]*Func{}
	for _, f := range fs {
		s.facts[f] = newShareFacts(p, f, s)
		s.taken[f] = make([]bool, len(f.Params))
		s.mutates[f] = make([]bool, len(f.Params))
		for _, g := range p.callees(f) {
			callers[g] = append(callers[g], f)
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == OpFnItem || in.Op == OpBind || in.Op == OpClosure {
					s.valued[in.Ent] = true
				}
			}
		}
	}
	settle(fs, callers, func(f *Func) bool { return s.facts[f].noteMutates() })
	for _, f := range fs {
		s.facts[f].settleLent()
	}
	settle(fs, callers, func(f *Func) bool {
		w := s.facts[f]
		w.memo, w.viewers = nil, nil
		if s.retView[f] || !w.returnsView() {
			return false
		}
		s.retView[f] = true
		return true
	})
	for _, f := range fs {
		s.facts[f].decideBindings()
	}
	settle(fs, callers, func(f *Func) bool { return s.facts[f].noteTaken() })
	return s
}

// A summary only grows, so a body is revisited only when one of its callees' changed.
func settle(fs []*Func, callers map[*Func][]*Func, step func(*Func) bool) {
	work := slices.Clone(fs)
	queued := map[*Func]bool{}
	for _, f := range fs {
		queued[f] = true
	}
	for len(work) > 0 {
		f := work[0]
		work = work[1:]
		queued[f] = false
		if !step(f) {
			continue
		}
		for _, c := range callers[f] {
			if !queued[c] {
				queued[c] = true
				work = append(work, c)
			}
		}
	}
}

func (p *Program) callees(f *Func) []*Func {
	var out []*Func
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if g := p.ByEnt[in.Ent]; in.Op == OpCall && g != nil && !slices.Contains(out, g) {
				out = append(out, g)
			}
		}
	}
	return out
}

// Only a body called by name alone has its callers copy such a result; any other copies it itself.
func (w *shareFacts) returnsView() bool {
	f := w.f
	if f.Ent == 0 || f.Async || w.p.R.Entity(f.Ent).Kind != sem.EntFn || w.sum.valued[f.Ent] {
		return false
	}
	for _, blk := range w.blocks {
		if blk.Term.Op != TermReturn || len(blk.Term.Args) == 0 || w.aliasOfNamed(blk.Term.Args[0]) {
			continue
		}
		for _, r := range w.rootsOf(blk.Term.Args[0]).refs {
			if r.inner && w.p.lentLocal(f, r.l) {
				return true
			}
		}
	}
	return false
}

func (s *shareSummary) lendsResult(in Inst, i int) bool {
	p := s.p
	if in.Op == OpCallValue {
		return in.Ent != 0 && i == 1 && p.inPlace(in, i)
	}
	if g := p.ByEnt[in.Ent]; g != nil {
		return s.retView[g] && i < len(g.Params) && p.lentLocal(g, g.Params[i])
	}
	return !p.IsStdPrimitive(in.Ent) && (p.inPlace(in, i) || p.lentParam(in, i))
}

func (w *shareFacts) noteTaken() bool {
	f, changed := w.f, false
	open := func(rs rootSet) bool {
		for _, r := range rs.refs {
			if i := slices.Index(f.Params, r.l); i >= 0 && !w.p.lentLocal(f, r.l) && !w.sum.taken[f][i] {
				return true
			}
		}
		return false
	}
	mark := func(rs rootSet) {
		for _, r := range rs.refs {
			if i := slices.Index(f.Params, r.l); i >= 0 && !w.p.lentLocal(f, r.l) && !w.sum.taken[f][i] {
				w.sum.taken[f][i], changed = true, true
			}
		}
	}
	if !slices.ContainsFunc(f.Params, func(prm LocalID) bool { return open(rootSet{refs: []rootRef{{l: prm}}}) }) {
		return false
	}
	for _, writes := range []map[LocalID][]Point{w.own, w.inner} {
		for l := range writes {
			if !w.sealed(l) {
				mark(w.rootsOf(l))
			}
		}
	}
	for b, blk := range w.blocks {
		for k, in := range blk.Insts {
			for i, a := range in.Args {
				kind := w.p.sinkAt(f, in, i, w.sum)
				if kind == noSink || w.sealed(a) || !open(w.rootsOf(a)) || kind == takesArg && w.aliasOfNamed(a) || w.shared(a, Point{b, k}) {
					continue
				}
				mark(w.rootsOf(a))
			}
		}
		if blk.Term.Op == TermReturn && len(blk.Term.Args) > 0 {
			if a := blk.Term.Args[0]; !w.sealed(a) && open(w.rootsOf(a)) && !w.aliasOfNamed(a) && !w.returnCopies(a) {
				mark(w.rootsOf(a))
			}
		}
	}
	return changed
}
