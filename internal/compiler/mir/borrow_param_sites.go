package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// For moves, a value counts once it may keep an exclusive borrow, and a write back under the parameter does not.
func (c *borrowCheck) collectStoreSites(k paramKey, from map[LocalID]derivation) []paramStore {
	c.siteRuns++
	g := k.g
	holdsMut := func(l LocalID) bool {
		if _, ok := from[l]; !ok {
			return false
		}
		switch k.kind {
		case siteMoves:
			return !c.plainData(g.Locals[l].Type)
		case siteOut:
			return c.carries(g.Locals[l].Type)
		}
		return c.mayStoreThrough(g.Locals[l].Type)
	}
	var out []paramStore
	add := func(d derivation, below paramStore) {
		s := paramStore{path: d.path, beyond: d.beyond || below.beyond, via: d.via || below.via, callee: below.callee}
		if !d.cut {
			s.path = d.path.with(below.path...)
		}
		if !slices.ContainsFunc(out, s.same) {
			out = append(out, s)
		}
	}
	defs := soleDefs(g)
	for _, blk := range g.Blocks {
		for _, a := range blk.Term.Args {
			if k.kind != siteSelf && holdsMut(a) {
				add(from[a], paramStore{via: true})
			}
		}
		for _, in := range blk.Insts {
			if k.kind == siteSelf {
				c.instSelfSources(g, in, from, defs, holdsMut, add)
			} else {
				c.instStoreSites(k, in, from, defs, holdsMut, add)
			}
		}
	}
	return out
}

func sameSites(a, b []paramStore) bool {
	return len(a) == len(b) && !slices.ContainsFunc(a, func(s paramStore) bool {
		return !slices.ContainsFunc(b, s.same)
	})
}

// instStoreSites adds the stores in may make through a value reached from the parameter: a write of a tracked value
// under it, or a `&mut` from it handed on to code that may store, or put anywhere else.
func (c *borrowCheck) instStoreSites(key paramKey, in Inst, from map[LocalID]derivation, defs map[LocalID]*Inst, holdsMut func(LocalID) bool, add func(derivation, paramStore)) {
	g, moves := key.g, key.kind == siteMoves || key.kind == siteOut
	switch {
	case in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet:
		v := in.Args[len(in.Args)-1]
		_, under := from[in.Args[0]]
		if under && !moves && c.carries(g.Locals[v].Type) {
			add(from[in.Args[0]], paramStore{})
		}
		if holdsMut(v) && !(moves && under) {
			add(from[v], paramStore{via: true})
		}
		return
	case derives[in.Op], inertOp(in.Op):
		return
	}
	for k, a := range in.Args {
		if !holdsMut(a) {
			continue
		}
		// Calling a closure runs its body on what it captured.
		if in.Op == OpCallValue && k == 0 {
			add(from[a], paramStore{via: true})
			continue
		}
		if h := c.bodyCallee(in); h != nil {
			for _, s := range c.paramSites(paramKey{g: h, i: k, kind: key.kind}) {
				for _, mapped := range c.relaySite(g, defs, s, in.Args, key.kind, 8) {
					add(from[a], mapped)
				}
			}
			continue
		}
		// A callee without a body here can only store what it is handed, and an index cannot hold a closure.
		other := func(b LocalID) bool {
			_, same := from[b]
			return b != a && c.carries(g.Locals[b].Type) && !(moves && same)
		}
		if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpBuiltin || slices.ContainsFunc(in.Args, other) {
			callback := slices.ContainsFunc(in.Args, func(b LocalID) bool { return b != a && c.typeHolds(g.Locals[b].Type, c.callableNode) })
			site := paramStore{via: in.Op != OpCall || callback || c.seesThrough(in.Ent, k) || c.viewType(g.Locals[a].Type)}
			if in.Op == OpCallValue {
				for _, target := range c.relayCallee(g, defs, in.Args[0], key.kind) {
					site.callee = target
					add(from[a], site)
				}
				continue
			}
			add(from[a], site)
		}
	}
}

// instSelfSources adds where a part of the parameter that in may store back under the parameter is read from: a write
// of it under the parameter (not a cell's write-back), or a hand-off of it beside a place from the parameter.
func (c *borrowCheck) instSelfSources(g *Func, in Inst, from map[LocalID]derivation, defs map[LocalID]*Inst, holdsMut func(LocalID) bool, add func(derivation, paramStore)) {
	derived := func(l LocalID) bool { _, ok := from[l]; return ok }
	part := func(l LocalID) bool { return derived(l) && c.carries(g.Locals[l].Type) }
	switch {
	case in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet:
		v := in.Args[len(in.Args)-1]
		if d := defs[v]; derived(in.Args[0]) && part(v) && (d == nil || d.Op != OpCellTake && d.Op != OpCellGet) {
			add(from[v], paramStore{})
		}
		return
	case derives[in.Op], inertOp(in.Op):
		return
	}
	h := c.bodyCallee(in)
	for k, a := range in.Args {
		if !part(a) {
			continue
		}
		if h != nil {
			for _, s := range c.paramSites(paramKey{g: h, i: k, kind: siteSelf}) {
				add(from[a], s)
			}
		}
		for j, b := range in.Args {
			if j != k && holdsMut(b) && (h == nil || c.paramStores(h, j)) {
				add(from[a], paramStore{})
				break
			}
		}
	}
}

// bodyCallee is the body a call or closure literal runs, when the summaries can read it.
func (c *borrowCheck) bodyCallee(in Inst) *Func {
	if h := c.p.ByEnt[in.Ent]; in.Op == OpClosure || in.Op == OpCall && h != nil && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
		return h
	}
	return nil
}
