package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// detaches reports whether running ent may leave a Future or Task running after it returns: it, something it calls or
// builds around one, or a callable it forwards, is a bodiless taker that may run one (not runToCompletion), or hands one on unseen.
func (c *borrowCheck) detaches(ent sem.EntityID) bool {
	return c.callGraphMay(ent, c.offloads().detach, func(e sem.EntityID) (bool, []sem.EntityID) {
		_, out := c.spawnEdges(e)
		prim := c.p.R.Entity(e).Kind == sem.EntFn && c.p.R.Fn(e).Body == 0 && !c.p.R.IsTaskRunToCompletion(e)
		g := c.p.ByEnt[e]
		fwd, via := c.forwardsIn(g)
		unseen, held := c.handsToUnknown(g)
		return prim && c.takesAsync(e) && c.mayRun(e) || unseen || fwd, append(append(out, via...), held...)
	})
}

// argDetaches reports whether in may leave argument k in a task that outlives it: a callee that detaches and takes it
// in a slot that can keep a Future or Task, a callee it cannot see, or, for an await, the body of a future it holds.
func (c *borrowCheck) argDetaches(in Inst, k int, held []sem.EntityID) bool {
	switch {
	case in.Op == OpAwait:
		return slices.ContainsFunc(held, c.detaches)
	case in.Op == OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn:
		return c.slotKeepsHandle(c.f, in, k) && c.detaches(in.Ent)
	case in.Op == OpCallValue && c.calleeClosure(c.f, in) != 0:
		return c.slotKeepsHandle(c.f, in, k) && c.detaches(c.calleeClosure(c.f, in))
	}
	return c.slotKeepsHandle(c.f, in, k)
}

// handsToUnknown reports g calling a value from elsewhere with a Future or Task (its own parameters are its callers'),
// and lists what else may run what g holds: fns and closures its value calls reach, closures it builds around one.
func (c *borrowCheck) handsToUnknown(g *Func) (bool, []sem.EntityID) {
	if g == nil {
		return false, nil
	}
	fs := c.bodyFacts(g)
	var held []sem.EntityID
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpClosure && slices.ContainsFunc(in.Args, func(a LocalID) bool { return c.typeRuns(g.Locals[a].Type, runsHandle) }) {
				held = append(held, in.Ent)
			}
			if in.Op != OpCall && in.Op != OpCallValue {
				continue
			}
			ents, _, unseen := c.calleeOf(g, fs, in)
			held = append(held, ents...)
			for k := range in.Args {
				if unseen && c.slotKeepsHandle(g, in, k) {
					return true, nil
				}
			}
		}
	}
	return false, held
}

// slotKeepsHandle reports whether argument k of call in, in body g, goes to a parameter whose declared type can keep a
// Future or Task, by the argument's own type where the parameter is unknown; the callee value itself is not handed on.
func (c *borrowCheck) slotKeepsHandle(g *Func, in Inst, k int) bool {
	t := g.Locals[in.Args[k]].Type
	switch in.Op {
	case OpCallValue:
		if k == 0 {
			return false
		}
		if params, known := c.calleeParams(g.Locals[in.Args[0]].Type); known && k-1 < len(params) {
			t = params[k-1]
		}
	case OpCall:
		if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
			info := c.p.R.Fn(in.Ent)
			params := info.Params
			if info.Recv != sem.RecvNone {
				params = append([]sem.EntityID{info.SelfParam}, params...)
			}
			if k < len(params) && params[k] != 0 {
				t = c.p.R.Entity(params[k]).Type
			}
		}
	default:
		return false
	}
	return c.typeRuns(t, runsHandle)
}

func (c *borrowCheck) calleeClosure(g *Func, in Inst) sem.EntityID {
	if t := c.p.R.Types.Node(g.Locals[in.Args[0]].Type); t.Kind == sem.KClosure && t.Ent != 0 && c.p.R.Entity(t.Ent).Kind == sem.EntClosure {
		return t.Ent
	}
	return 0
}

// holdsHandle reports whether a value of t, held by value, can keep a Future or Task: one inside it, or a type parameter,
// open type, interface or closure that may; a borrow can neither be kept nor spawned, and a plain `fn` captures no move-only value.
func (c *borrowCheck) holdsHandle(t sem.TypeID) bool {
	return c.p.R.Types.Kind(t) != sem.KRef && c.typeHolds(t, func(t sem.TypeID) bool {
		switch c.p.R.Types.Kind(t) {
		case sem.KParam, sem.KVar, sem.KIface, sem.KClosure:
			return true
		}
		return c.p.R.Types.Kind(t) == sem.KNamed && c.p.R.Types.Node(t).Ent == c.p.R.Types.FutureEnt() || c.p.R.IsTaskType(t)
	})
}
