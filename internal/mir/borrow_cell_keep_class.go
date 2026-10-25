package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// keeperClass is what a store into by-value shared storage reaches here: every local the value flow links to it
// through a common origin, and whether that origin is a parameter, whose storage outlives the body.
type keeperClass struct {
	locals  []LocalID
	escapes bool
}

// keeperGraph is the value flow's edges between locals able to hold or reach a keepable value, flow-insensitively.
type keeperGraph struct {
	succ, pred map[LocalID][]LocalID
	memo       map[LocalID]keeperClass
}

// keeperOf links a to its origins and from them to everything derived from them, so a copy taken before the store
// sees it too.
func (c *borrowCheck) keeperOf(a LocalID) keeperClass {
	g := c.keepers()
	if k, ok := g.memo[a]; ok {
		return k
	}
	back := reachLocals([]LocalID{a}, g.pred)
	k := keeperClass{locals: reachLocals(back, g.succ)}
	k.escapes = slices.ContainsFunc(back, func(l LocalID) bool { return slices.Contains(c.f.Params, l) })
	g.memo[a] = k
	return k
}

func (c *borrowCheck) keepers() *keeperGraph {
	if c.keeperState != nil {
		return c.keeperState
	}
	g := &keeperGraph{succ: map[LocalID][]LocalID{}, pred: map[LocalID][]LocalID{}, memo: map[LocalID]keeperClass{}}
	node := func(l LocalID) bool {
		t := c.f.Locals[l].Type
		if c.p.R.Types.Kind(t) == sem.KRef {
			t = c.p.R.Types.Node(t).Elem
		}
		return l != 0 && c.keepable(t)
	}
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			dsts, srcs := c.flowEdges(in)
			for _, d := range dsts {
				for _, s := range srcs {
					if d != s && node(d) && node(s) {
						g.succ[s], g.pred[d] = append(g.succ[s], d), append(g.pred[d], s)
					}
				}
			}
		}
	}
	c.keeperState = g
	return g
}

func reachLocals(from []LocalID, edges map[LocalID][]LocalID) []LocalID {
	out := slices.Clone(from)
	for i := 0; i < len(out); i++ {
		for _, n := range edges[out[i]] {
			if !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}

// storePlaces are the locals a callee's store into argument a reaches: what a lent `&mut` points into and, for shared
// storage, its keeper class and, when that storage outlives the body, the task scope.
func (c *borrowCheck) storePlaces(a LocalID) []LocalID {
	var out []LocalID
	for _, p := range c.lentMutPlaces(a) {
		out = append(out, p.root, p.via)
	}
	return append(out, c.sharedPlaces(a)...)
}

// sharedPlaces is a's keeper class when it reaches shared storage, with the task scope when that outlives the body.
func (c *borrowCheck) sharedPlaces(a LocalID) []LocalID {
	if !c.sharedStorage(c.f.Locals[a].Type) {
		return nil
	}
	k := c.keeperOf(a)
	if k.escapes {
		return append(slices.Clone(k.locals), c.spawnScope())
	}
	return k.locals
}

// keptArgs are the arguments of call in holding shared storage its callee may store into: by its summary, or every
// one for a callee this body cannot see.
func (c *borrowCheck) keptArgs(in Inst, s flowSets) []int {
	ents := c.knownCallees(c.f, s, in)
	var out []int
	for k, a := range in.Args {
		if !c.sharedStorage(c.f.Locals[a].Type) {
			continue
		}
		kept := ents == nil
		for _, e := range ents {
			sum, ok := c.summaryOf(e)
			kept = kept || !ok || slices.ContainsFunc(c.boundParams([]sem.EntityID{e}, in, k), func(q sem.EntityID) bool { return len(sum.stores[q]) > 0 })
		}
		if kept {
			out = append(out, k)
		}
	}
	return out
}
