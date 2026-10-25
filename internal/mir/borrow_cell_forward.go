package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// declaredParams are the parameter entities of a known fn or closure, in the order a call passes its arguments; a
// method value's are its method's, receiver first.
func (c *borrowCheck) declaredParams(ent sem.EntityID) []sem.EntityID {
	if m := c.methodOf(ent); m != 0 {
		ent = m
	}
	switch c.p.R.Entity(ent).Kind {
	case sem.EntFn:
		info := c.p.R.Fn(ent)
		if info.Recv != sem.RecvNone {
			return append([]sem.EntityID{info.SelfParam}, info.Params...)
		}
		return info.Params
	case sem.EntClosure:
		return c.p.R.Closure(ent).Params
	}
	return nil
}

// callsParam reports whether its fn or closure may call callable parameter p with an argument that can keep a Future or
// Task: a call of a value that may hold p, p handed to a closure or an unseen callee, or to a callee's such parameter.
func (c *borrowCheck) callsParam(p sem.EntityID) bool {
	return c.callGraphMay(p, c.offloads().calls, func(p sem.EntityID) (bool, []sem.EntityID) {
		g := c.p.ByEnt[c.p.R.Entity(p).Parent]
		if g == nil || !c.typeRuns(c.p.R.Entity(p).Type, runsCallable) {
			return false, nil
		}
		fs, pf := c.bodyFacts(g), uint32(p)
		direct := false
		var out []sem.EntityID
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				ents, params, unseen := c.calleeOf(g, fs, in)
				for k, a := range in.Args {
					switch {
					case !fs[a][pf]:
					case in.Op == OpClosure:
						direct = true
					case in.Op == OpCallValue && k == 0:
						for j := 1; j < len(in.Args); j++ {
							direct = direct || c.slotKeepsHandle(g, in, j)
						}
					case in.Op == OpCall || in.Op == OpCallValue || in.Op == OpFutureNew:
						out = append(out, c.boundParams(ents, in, k)...)
						direct = direct || unseen || len(params) > 0
					}
				}
			}
		}
		return direct, out
	})
}

// calleeParam is the parameter of a known callee that argument k of in, in body g with sole definitions defs, binds, or 0.
func (c *borrowCheck) calleeParam(g *Func, defs map[LocalID]*Inst, in Inst, k int) sem.EntityID {
	var ents []sem.EntityID
	switch {
	case in.Op == OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn:
		ents = []sem.EntityID{in.Ent}
	case in.Op == OpCallValue && c.calleeClosure(g, in) != 0:
		ents = []sem.EntityID{c.calleeClosure(g, in)}
	case in.Op == OpFutureNew:
		if d := defs[in.Args[0]]; d != nil && d.Op == OpFnItem {
			ents = []sem.EntityID{d.Ent}
		}
	}
	if qs := c.boundParams(ents, in, k); len(qs) > 0 {
		return qs[0]
	}
	return 0
}

// forwardDetaches reports whether a known callee of in may call one of its parameters with a handle while the argument
// bound there is a callable that detaches, or one this body cannot see into.
func (c *borrowCheck) forwardDetaches(in Inst, s flowSets) bool {
	defs := map[LocalID]*Inst{}
	if in.Op == OpFutureNew {
		defs[in.Args[0]] = c.definition(in.Args[0])
	}
	for k, a := range in.Args {
		if q := c.calleeParam(c.f, defs, in, k); q == 0 || !c.callsParam(q) {
			continue
		}
		held, _ := c.splitOffloadFacts(s[a], a)
		if len(held) == 0 || s[a][opaqueFact] || slices.ContainsFunc(held, c.detaches) {
			return true
		}
	}
	return false
}

// opaqueStep gives what a call may hand back or store into a lent place: a callable result by value is from elsewhere,
// and a known callee's stores and result come from its summary (calleeStores); one it cannot see stores the unknown.
func (c *borrowCheck) opaqueStep(in Inst, s flowSets, put func(LocalID, uint32)) {
	if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpAwait && in.Op != OpFutureNew {
		return
	}
	dst := c.f.Locals[in.Dst].Type
	if in.Op != OpFutureNew && c.p.R.Types.Kind(dst) != sem.KRef && c.typeRuns(dst, runsCallable) {
		put(in.Dst, opaqueFact)
	}
	lent := func(k int, facts []uint32) {
		for _, l := range c.storePlaces(in.Args[k]) {
			for _, f := range facts {
				put(l, f)
			}
		}
	}
	result := func(facts []uint32) {
		for _, f := range facts {
			put(in.Dst, f)
		}
	}
	if in.Op == OpAwait || c.knownCallees(c.f, s, in) == nil {
		for k, a := range in.Args {
			if c.keepsThrough(c.f, a) && c.typeRuns(c.f.Locals[a].Type, runsCallable) {
				lent(k, []uint32{opaqueFact})
			}
		}
		if c.typeRuns(dst, runsCallable) {
			put(in.Dst, opaqueFact)
		}
		return
	}
	argFacts := func(a LocalID) []uint32 {
		var out []uint32
		for f := range s[a] {
			out = append(out, f)
		}
		return out
	}
	c.calleeStores(c.f, s, in, argFacts, lent, result)
}

// forwardsIn applies forwardDetaches to g's own calls by g's body facts: a fn or closure bound to a parameter its callee
// calls with a handle is an edge, one from elsewhere makes g detach, and g's own parameter is left to g's callers.
func (c *borrowCheck) forwardsIn(g *Func) (bool, []sem.EntityID) {
	if g == nil {
		return false, nil
	}
	fs := c.bodyFacts(g)
	var out []sem.EntityID
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			ents, _, _ := c.calleeOf(g, fs, in)
			for k, a := range in.Args {
				if !slices.ContainsFunc(c.boundParams(ents, in, k), c.callsParam) {
					continue
				}
				if len(fs[a]) == 0 || fs[a][opaqueFact] {
					return true, nil
				}
				for f := range fs[a] {
					if kind := c.p.R.Entity(sem.EntityID(f)).Kind; kind == sem.EntFn || kind == sem.EntClosure {
						out = append(out, sem.EntityID(f))
					} else if kind != sem.EntParam {
						return true, nil
					}
				}
			}
		}
	}
	return false, out
}
