package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

type shareSummary struct {
	p       *planner
	valued  map[sem.EntityID]bool
	retView map[*midir.Func]bool
	taken   map[*midir.Func][]bool
	mutates map[*midir.Func][]bool
	facts   map[*midir.Func]*shareFacts
	sealed  map[sem.TypeID]bool
}

func (p *planner) planShares(fs []*midir.Func) *shareSummary {
	s := &shareSummary{p: p, valued: map[sem.EntityID]bool{}, retView: map[*midir.Func]bool{}, taken: map[*midir.Func][]bool{},
		mutates: map[*midir.Func][]bool{}, facts: map[*midir.Func]*shareFacts{}, sealed: map[sem.TypeID]bool{}}
	callers := map[*midir.Func][]*midir.Func{}
	for _, f := range fs {
		s.facts[f] = newShareFacts(p, f, s)
		s.taken[f] = make([]bool, len(f.Params))
		s.mutates[f] = make([]bool, len(f.Params))
		for _, g := range p.callees(f) {
			callers[g] = append(callers[g], f)
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == midir.OpFnItem || in.Op == midir.OpBind || in.Op == midir.OpClosure {
					s.valued[in.Ent] = true
				}
			}
		}
	}
	settle(fs, callers, func(f *midir.Func) bool { return s.facts[f].noteMutates() })
	for _, f := range fs {
		s.facts[f].settleLent()
	}
	settle(fs, callers, func(f *midir.Func) bool {
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
	settle(fs, callers, func(f *midir.Func) bool { return s.facts[f].noteTaken() })
	return s
}

// A summary only grows, so a body is revisited only when one of its callees' changed.
func settle(fs []*midir.Func, callers map[*midir.Func][]*midir.Func, step func(*midir.Func) bool) {
	work := slices.Clone(fs)
	queued := map[*midir.Func]bool{}
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

func (p *planner) callees(f *midir.Func) []*midir.Func {
	var out []*midir.Func
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if g := p.ByEnt[in.Ent]; in.Op == midir.OpCall && g != nil && !slices.Contains(out, g) {
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
		if blk.Term.Op != midir.TermReturn || len(blk.Term.Args) == 0 || w.aliasOfNamed(blk.Term.Args[0]) {
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

func (s *shareSummary) lendsResult(in midir.Inst, i int) bool {
	p := s.p
	if in.Op == midir.OpCallValue {
		return in.Ent != 0 && i == 1 && p.InPlace(in, i)
	}
	if g := p.ByEnt[in.Ent]; g != nil {
		return s.retView[g] && i < len(g.Params) && p.lentLocal(g, g.Params[i])
	}
	return !ownquery.IsStdPrimitive(p.R, in.Ent) && (p.InPlace(in, i) || p.LentParam(in, i))
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
	if !slices.ContainsFunc(f.Params, func(prm midir.LocalID) bool { return open(rootSet{refs: []rootRef{{l: prm}}}) }) {
		return false
	}
	for _, writes := range []map[midir.LocalID][]Point{w.own, w.inner} {
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
		if blk.Term.Op == midir.TermReturn && len(blk.Term.Args) > 0 {
			if a := blk.Term.Args[0]; !w.sealed(a) && open(w.rootsOf(a)) && !w.aliasOfNamed(a) && !w.returnCopies(a) {
				mark(w.rootsOf(a))
			}
		}
	}
	return changed
}
