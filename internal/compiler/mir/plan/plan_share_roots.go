package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type rootRef struct {
	l          midir.LocalID
	own, inner bool
}

// other is storage this body cannot follow: a global, or a cell a closure may write at any call.
type rootSet struct {
	refs  []rootRef
	other bool
}

func (s *rootSet) add(r rootRef) {
	for i := range s.refs {
		if s.refs[i].l == r.l {
			s.refs[i].own = s.refs[i].own || r.own
			s.refs[i].inner = s.refs[i].inner || r.inner
			return
		}
	}
	s.refs = append(s.refs, r)
}

func (s *rootSet) merge(o rootSet) {
	for _, r := range o.refs {
		s.add(r)
	}
	s.other = s.other || o.other
}

type rootKey struct {
	l     midir.LocalID
	inner bool
}

// Until the bindings are decided, every binding counts as sharing its source.
func (w *shareFacts) rootsOf(v midir.LocalID) rootSet {
	if rs, ok := w.memo[v]; ok {
		return rs
	}
	rs := w.collect(v, false, map[rootKey]bool{})
	if w.memo == nil {
		w.memo = map[midir.LocalID]rootSet{}
	}
	w.memo[v] = rs
	return rs
}

func (w *shareFacts) collect(v midir.LocalID, inner bool, seen map[rootKey]bool) rootSet {
	var rs rootSet
	if seen[rootKey{v, inner}] {
		return rs
	}
	seen[rootKey{v, inner}] = true
	if w.f.Locals[v].Ent != 0 {
		rs.add(rootRef{l: v, own: !inner, inner: inner})
		rs.other = w.untracked(v)
		for _, at := range w.binds[v] {
			if !w.decided || !w.copyAt[at] {
				rs.merge(w.collect(w.inst(at).Args[0], inner, seen))
			}
		}
		return rs
	}
	for _, in := range w.defs[v] {
		switch {
		case projects(*in):
			rs.merge(w.collect(in.Args[0], true, seen))
		case in.Op == midir.OpShare && in.Copy, in.Op == midir.OpCopy && in.Share:
		case passesOn(in.Op):
			rs.merge(w.collect(in.Args[0], inner, seen))
		case in.Op == midir.OpGlobalGet:
			rs.other = true
		case in.Op == midir.OpCall, in.Op == midir.OpCallValue:
			rs.merge(w.callRoots(*in, seen))
		}
	}
	return rs
}

// A value call's callee is unknown, so every reference it is handed counts.
func (w *shareFacts) callRoots(in midir.Inst, seen map[rootKey]bool) rootSet {
	var rs rootSet
	for i, a := range in.Args {
		byRef := in.Op == midir.OpCallValue && i > 0 && w.p.R.Types.Kind(w.f.Locals[a].Type) == sem.KRef
		if byRef || w.sum.lendsResult(in, i) {
			rs.merge(w.collect(a, true, seen))
		}
	}
	return rs
}

// A closure's captures stay in its environment for the next call.
func (p *planner) lentLocal(f *midir.Func, v midir.LocalID) bool {
	i := slices.Index(f.Params, v)
	if i < 0 {
		return false
	}
	l := f.Locals[v]
	return l.Borrowed || l.Cell || p.R.Types.Kind(l.Type) == sem.KRef || i < p.captures(f)
}

func (p *planner) captures(f *midir.Func) int {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntClosure {
		return 0
	}
	return len(p.R.AggFields(p.R.Types.Closure(f.Ent)))
}

func (w *shareFacts) untracked(v midir.LocalID) bool {
	l := w.f.Locals[v]
	return l.Cell && !l.Borrowed && w.p.R.Types.Kind(l.Type) != sem.KRef
}

// PlanRC already copies a named local's own value wherever it is taken over or returned.
func (w *shareFacts) aliasOfNamed(v midir.LocalID) bool {
	for range w.nLocals {
		if w.f.Locals[v].Ent != 0 {
			return true
		}
		ds := w.defs[v]
		if len(ds) != 1 || len(ds[0].Args) != 1 {
			return false
		}
		if in := ds[0]; in.Op != midir.OpAlias && (in.Op != midir.OpCopy || in.Share) {
			return false
		}
		v = ds[0].Args[0]
	}
	return false
}

func (w *shareFacts) viewersOf(l midir.LocalID) []midir.LocalID {
	if w.viewers == nil {
		w.viewers = map[midir.LocalID][]midir.LocalID{}
		for i := range w.nLocals {
			v := midir.LocalID(i)
			if w.p.R.Types.Kind(w.f.Locals[v].Type) == sem.KPrim {
				continue
			}
			for _, r := range w.rootsOf(v).refs {
				if r.l != v {
					w.viewers[r.l] = append(w.viewers[r.l], v)
				}
			}
		}
	}
	return w.viewers[l]
}
