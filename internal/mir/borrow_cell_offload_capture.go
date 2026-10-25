package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// captureStep follows work closures keep by themselves: one built with a body that may detach sends what it captures to
// the task scope, and what a closure that may keep values is built with or run on goes to whatever keeps that closure.
func (c *borrowCheck) captureStep(in Inst, s flowSets, put func(LocalID, uint32)) {
	switch in.Op {
	case OpClosure:
		if c.p.R.Entity(in.Ent).Kind != sem.EntClosure {
			return
		}
		if c.detaches(in.Ent) {
			for _, a := range in.Args {
				held, handed := c.splitOffloadFacts(s[a], a)
				for _, f := range handed {
					put(c.spawnScope(), f)
				}
				if c.mayOffload(in.Ent) || slices.ContainsFunc(held, c.mayOffload) {
					c.handOff(in, held, c.spawnScope(), put)
				}
			}
		}
		if c.mayKeep(in.Ent) {
			c.keepInto([]sem.EntityID{in.Ent}, in.Args, s, put)
		}
	case OpCall, OpCallValue, OpAwait:
		var run []sem.EntityID
		for k, a := range in.Args {
			if in.Op == OpCallValue && k == 0 || c.argRunnable(in, k) {
				held, _ := c.splitOffloadFacts(s[a], a)
				for _, cl := range held {
					if c.p.R.Entity(cl).Kind != sem.EntClosure {
						continue
					}
					if c.mayKeep(cl) && !slices.Contains(run, cl) {
						run = append(run, cl)
					}
					// Work it hands off itself and leaves running reaches what it captures, so it stays in flight as cl.
					if c.mayOffload(cl) && c.detaches(cl) {
						c.handOff(in, []sem.EntityID{cl}, c.spawnScope(), put)
					}
				}
			}
		}
		if len(run) > 0 {
			c.keepInto(run, in.Args, s, put)
		}
	}
}

// keepInto puts what the given values carry into every value holding one of the closures, and into the cells and
// borrowed locals their literals here share.
func (c *borrowCheck) keepInto(cls []sem.EntityID, from []LocalID, s flowSets, put func(LocalID, uint32)) {
	var dsts []LocalID
	for l, fs := range s {
		if slices.ContainsFunc(cls, func(cl sem.EntityID) bool { return fs[uint32(cl)] }) {
			dsts = append(dsts, l)
		}
	}
	for _, cl := range cls {
		caps := c.p.R.Closure(cl).Captures
		for _, site := range c.closureSites(cl) {
			for j, a := range site {
				if j < len(caps) && (caps[j].Mode == sem.CapCell || caps[j].Mode == sem.CapBorrow) {
					dsts = append(dsts, a)
				}
			}
		}
	}
	var facts []uint32
	for _, a := range from {
		for f := range s[a] {
			facts = append(facts, f)
		}
	}
	for _, d := range dsts {
		for _, f := range facts {
			put(d, f)
		}
	}
}

// mayKeep reports whether running closure cl may keep a value it is given or holds in what it captures: its body may
// store into a capture, calls a callee value that may be such a closure, or runs a closure that may.
func (c *borrowCheck) mayKeep(cl sem.EntityID) bool {
	return c.callGraphMay(cl, c.offloads().keep, func(e sem.EntityID) (bool, []sem.EntityID) {
		g := c.p.ByEnt[e]
		if c.p.R.Entity(e).Kind != sem.EntClosure {
			return false, nil
		}
		if g == nil {
			return true, nil
		}
		direct := false
		for j := range c.p.R.Closure(e).Captures {
			direct = direct || c.paramStores(g, j)
		}
		var out []sem.EntityID
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if in.Op == OpCallValue && c.calleeClosure(g, in) == 0 && len(in.Args) > 1 && c.p.R.Types.Kind(g.Locals[in.Args[0]].Type) != sem.KFn {
					direct = true
				}
				if in.Op == OpCall || in.Op == OpCallValue || in.Op == OpClosure {
					for _, a := range in.Args {
						out = append(out, c.typeClosures(g.Locals[a].Type)...)
					}
				}
			}
		}
		return direct, out
	})
}

// closureSites are the captured locals of each literal of closure cl in this body.
func (c *borrowCheck) closureSites(cl sem.EntityID) [][]LocalID {
	var out [][]LocalID
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpClosure && in.Ent == cl {
				out = append(out, in.Args)
			}
		}
	}
	return out
}
