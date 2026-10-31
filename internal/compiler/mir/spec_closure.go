package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type funcPair struct{ fn, gen *midir.Func }

// The copies' closure types join sub, which the copy of the outer body reads.
func (s *specializer) closureClones(g *midir.Func, sub map[sem.EntityID]sem.TypeID) ([]funcPair, map[sem.EntityID]sem.EntityID, bool) {
	all := s.withMethodValues(s.closuresBuilt(g), g)
	needs := map[sem.EntityID]bool{}
	for changed := true; changed; {
		changed = false
		for _, c := range all {
			if !needs[c] && s.closureTouches(c, sub, needs) {
				needs[c], changed = true, true
			}
		}
	}
	ents := map[sem.EntityID]sem.EntityID{}
	for len(ents) < len(needs) {
		progress := false
		for _, c := range all {
			if _, done := ents[c]; done || !needs[c] || !s.depsMinted(c, needs, ents) {
				continue
			}
			if body := s.p.ByEnt[c]; body != nil && (body.Async || body.Suspends > 0) {
				return nil, nil, false
			}
			c2 := s.p.R.InstantiateClosure(c, sub)
			ents[c], sub[c] = c2, s.p.R.Types.Closure(c2)
			progress = true
		}
		if !progress {
			return nil, nil, false
		}
	}
	var out []funcPair
	for _, c := range all {
		if c2, ok := ents[c]; ok && s.p.ByEnt[c] != nil {
			body := s.p.ByEnt[c]
			out = append(out, funcPair{fn: s.p.cloneFor(body, c2, s.p.funcName(c), sub, ents), gen: body})
		}
	}
	return out, ents, true
}

func (s *specializer) closuresBuilt(g *midir.Func) []sem.EntityID {
	var out []sem.EntityID
	seen := map[sem.EntityID]bool{}
	var walk func(f *midir.Func)
	walk = func(f *midir.Func) {
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op != midir.OpClosure || s.p.R.Entity(in.Ent).Kind != sem.EntClosure || seen[in.Ent] {
					continue
				}
				seen[in.Ent] = true
				if body := s.p.ByEnt[in.Ent]; body != nil {
					out = append(out, in.Ent)
					walk(body)
				}
			}
		}
	}
	walk(g)
	return out
}

func (s *specializer) closureTouches(c sem.EntityID, sub map[sem.EntityID]sem.TypeID, needs map[sem.EntityID]bool) bool {
	info := s.p.R.Closure(c)
	if s.touches(info.Sig, sub, needs, 0) || s.touches(info.Ret, sub, needs, 0) {
		return true
	}
	for _, cp := range info.Captures {
		if s.touches(s.p.R.Entity(cp.Local).Type, sub, needs, 0) {
			return true
		}
	}
	for _, l := range s.bodyLocals(c) {
		if s.touches(l.Type, sub, needs, 0) {
			return true
		}
	}
	return false
}

func (s *specializer) touches(t sem.TypeID, sub map[sem.EntityID]sem.TypeID, needs map[sem.EntityID]bool, depth int) bool {
	if t == 0 || depth > 32 {
		return false
	}
	n := s.p.R.Types.Node(t)
	if n.Kind == sem.KParam || n.Kind == sem.KClosure {
		if _, ok := sub[n.Ent]; ok || needs[n.Ent] {
			return true
		}
	}
	if s.touches(n.Elem, sub, needs, depth+1) {
		return true
	}
	for _, a := range n.Args {
		if s.touches(a, sub, needs, depth+1) {
			return true
		}
	}
	return false
}

func (s *specializer) depsMinted(c sem.EntityID, needs map[sem.EntityID]bool, ents map[sem.EntityID]sem.EntityID) bool {
	for _, l := range s.envLocals(c) {
		for _, x := range s.closureTypes(l.Type, nil, 0) {
			if _, done := ents[x]; x != c && needs[x] && !done {
				return false
			}
		}
	}
	return true
}

func (s *specializer) closureTypes(t sem.TypeID, out []sem.EntityID, depth int) []sem.EntityID {
	if t == 0 || depth > 32 {
		return out
	}
	n := s.p.R.Types.Node(t)
	if n.Kind == sem.KClosure {
		out = append(out, n.Ent)
	}
	out = s.closureTypes(n.Elem, out, depth+1)
	for _, a := range n.Args {
		out = s.closureTypes(a, out, depth+1)
	}
	return out
}

// A closure still generic would box a type parameter's value in its body.
func (s *specializer) leftGeneric(fs []*midir.Func) bool {
	seen := map[sem.TypeID]bool{}
	for _, f := range fs {
		for _, l := range f.Locals {
			if s.genericClosure(l.Type, seen) {
				return true
			}
		}
	}
	return false
}
