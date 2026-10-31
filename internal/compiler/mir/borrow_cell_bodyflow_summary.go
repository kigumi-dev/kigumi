package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// calleeSummary is, in the callee's own facts, what a body puts into each parameter's place and what it returns.
type calleeSummary struct {
	stores map[sem.EntityID][]uint32
	result []uint32
}

// finishedFlow is h's bodyFlow; ok is false for no body here or one still being computed (a recursion), which its
// users take as storing and returning a value from elsewhere.
func (c *borrowCheck) finishedFlow(h *Func) (fs, kept flowSets, ok bool) {
	if h == nil {
		return nil, nil, false
	}
	fs, kept = c.bodyFlow(h)
	return fs, kept, fs != nil
}

// summaryOf is e's summary, its method's for a method value and by its declared shape for a bodiless fn; ok is false
// where finishedFlow is.
func (c *borrowCheck) summaryOf(e sem.EntityID) (calleeSummary, bool) {
	if m := c.methodOf(e); m != 0 {
		e = m
	}
	if c.p.R.Entity(e).Kind == sem.EntFn && c.p.R.Fn(e).Body == 0 {
		return c.primSummary(e), true
	}
	g := c.p.ByEnt[e]
	fs, kept, ok := c.finishedFlow(g)
	if !ok {
		return calleeSummary{}, false
	}
	sum := calleeSummary{stores: map[sem.EntityID][]uint32{}}
	for _, l := range g.Params {
		for f := range kept[l] {
			sum.stores[g.Locals[l].Ent] = append(sum.stores[g.Locals[l].Ent], f)
		}
	}
	for _, blk := range g.Blocks {
		if blk.Term.Op == TermReturn {
			for _, a := range blk.Term.Args {
				for f := range fs[a] {
					sum.result = append(sum.result, f)
				}
			}
		}
	}
	return sum, true
}

// intoCaller turns callee facts into the caller's: a parameter of e becomes what the argument bound to it holds, and a
// parameter of anything else a value from elsewhere.
func (c *borrowCheck) intoCaller(e sem.EntityID, facts []uint32, arg func(sem.EntityID) []uint32) []uint32 {
	var out []uint32
	for _, f := range facts {
		switch q := sem.EntityID(f); {
		case f != opaqueFact && c.p.R.Entity(q).Kind == sem.EntParam && slices.Contains(c.declaredParams(e), q):
			out = append(out, arg(q)...)
		case f != opaqueFact && c.p.R.Entity(q).Kind == sem.EntParam:
			out = append(out, opaqueFact)
		default:
			out = append(out, f)
		}
	}
	return out
}

// lendsPlace reports an argument a callee may store through: a `&mut` or a write-back cell.
func (c *borrowCheck) lendsPlace(g *Func, a LocalID) bool {
	_, mut := c.p.R.MutRefElem(g.Locals[a].Type)
	return mut || g.Locals[a].Cell
}

// keepsThrough is lendsPlace or an argument reaching shared storage, which a callee may store into by value.
func (c *borrowCheck) keepsThrough(g *Func, a LocalID) bool {
	return c.lendsPlace(g, a) || c.sharedStorage(g.Locals[a].Type)
}

// knownCallees are the callees of in this body can name: a fn item, a closure by type, or an async fn's future; nil for
// a call it cannot see into.
func (c *borrowCheck) knownCallees(g *Func, fs flowSets, in Inst) []sem.EntityID {
	switch in.Op {
	case OpCall, OpCallValue, OpFutureNew:
		if ents, params, unseen := c.calleeOf(g, fs, in); !unseen && len(params) == 0 {
			return ents
		}
	}
	return nil
}

// calleeStores gives the facts each lent argument and the result of call in take from the callee's stores and return,
// in g's facts via argFacts; an unseen or still-computing callee stores the unknown into each lent callable place.
func (c *borrowCheck) calleeStores(g *Func, fs flowSets, in Inst, argFacts func(LocalID) []uint32, lent func(int, []uint32), result func([]uint32)) {
	if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpFutureNew {
		return
	}
	opaqueAll := func() {
		for k, a := range in.Args {
			if c.keepsThrough(g, a) && c.typeRuns(g.Locals[a].Type, runsCallable) {
				lent(k, []uint32{opaqueFact})
			}
		}
	}
	ents := c.knownCallees(g, fs, in)
	if ents == nil {
		opaqueAll()
		return
	}
	for _, e := range ents {
		sum, ok := c.summaryOf(e)
		switch {
		case !ok && c.p.ByEnt[e] == nil:
		case !ok:
			opaqueAll()
			result([]uint32{opaqueFact})
		default:
			arg := func(q sem.EntityID) []uint32 {
				var out []uint32
				for k, a := range in.Args {
					if slices.Contains(c.boundParams([]sem.EntityID{e}, in, k), q) {
						out = append(out, argFacts(a)...)
					}
				}
				return out
			}
			for k := range in.Args {
				for _, q := range c.boundParams([]sem.EntityID{e}, in, k) {
					lent(k, c.intoCaller(e, sum.stores[q], arg))
				}
			}
			result(c.intoCaller(e, sum.result, arg))
		}
	}
}

// bodyCallStores applies calleeStores inside bodyFacts: a lent argument's places, through placeRoots, and the result.
func (c *borrowCheck) bodyCallStores(g *Func, defs map[LocalID][]*Inst, in Inst, fs flowSets, into func([]LocalID, bool, []uint32), put func(LocalID, uint32, bool)) {
	argFacts := func(a LocalID) []uint32 {
		var out []uint32
		for f := range fs[a] {
			out = append(out, f)
		}
		return out
	}
	c.calleeStores(g, fs, in, argFacts, func(k int, facts []uint32) {
		if a := in.Args[k]; c.keepsThrough(g, a) {
			roots, ok := c.placeRoots(g, defs, a)
			into(roots, ok, facts)
		}
	}, func(facts []uint32) {
		for _, f := range facts {
			put(in.Dst, f, false)
		}
	})
}
