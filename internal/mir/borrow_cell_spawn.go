package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// argSpawns reports whether in may start a task from argument i, which then runs until the body ends: an awaited
// future, which may spawn into the running task, or a callee that takes or reaches a Future or Task.
func (c *borrowCheck) argSpawns(in Inst, i int) bool {
	switch in.Op {
	case OpAwait:
		return true
	case OpCallValue:
		if t := c.p.R.Types.Node(c.f.Locals[in.Args[0]].Type); t.Kind == sem.KClosure && t.Ent != 0 && c.p.R.Entity(t.Ent).Kind == sem.EntClosure {
			return i > 0 && c.argRunnable(in, i) && c.maySpawn(t.Ent)
		}
		return i > 0 && c.argRunnable(in, i)
	case OpCall:
		if in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
			return true
		}
		return c.argRunnable(in, i) && c.maySpawn(in.Ent)
	}
	return false
}

// spawnStep leaves every closure a value holds in the task scope when the call may spawn it, or run it while it spawns.
func (c *borrowCheck) spawnStep(in Inst, s flowSets, put func(LocalID, uint32)) {
	if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpAwait {
		return
	}
	for k, a := range in.Args {
		var held []uint32
		for f := range s[a] {
			if f != dropFact {
				held = append(held, f)
			}
		}
		for _, cl := range c.typeClosures(c.f.Locals[a].Type) {
			held = append(held, uint32(cl))
		}
		if !c.argSpawns(in, k) && (!c.argRunnable(in, k) || !slices.ContainsFunc(held, func(f uint32) bool { return c.maySpawn(sem.EntityID(f)) })) {
			continue
		}
		for _, f := range held {
			put(c.spawnScope(), f)
		}
	}
}

// maySpawn reports whether running fn or closure ent may start a task: it, or something it calls, takes a Future or
// Task.
func (c *borrowCheck) maySpawn(ent sem.EntityID) bool {
	if c.spawnMemo == nil {
		c.spawnMemo = map[sem.EntityID]bool{}
	}
	return c.callGraphMay(ent, c.spawnMemo, c.spawnEdges)
}

// mayRun reports whether fn or closure ent may call or await a value it did not name: it, or something it calls,
// calls a parameter, field or callback, awaits, or is a declaration without a body that takes something runnable.
func (c *borrowCheck) mayRun(ent sem.EntityID) bool {
	if c.runMemo == nil {
		c.runMemo = map[sem.EntityID]bool{}
	}
	return c.callGraphMay(ent, c.runMemo, c.runEdges)
}

// callGraphMay settles edges' property over the call graph below ent as one fixpoint, so every member of a cycle
// gets the union.
func (c *borrowCheck) callGraphMay(ent sem.EntityID, memo map[sem.EntityID]bool, edges func(sem.EntityID) (bool, []sem.EntityID)) bool {
	if v, ok := memo[ent]; ok {
		return v
	}
	may, succ := map[sem.EntityID]bool{}, map[sem.EntityID][]sem.EntityID{}
	var nodes []sem.EntityID
	seen := map[sem.EntityID]bool{ent: true}
	for stack := []sem.EntityID{ent}; len(stack) > 0; {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if v, ok := memo[n]; ok {
			may[n] = v
			continue
		}
		nodes = append(nodes, n)
		direct, targets := edges(n)
		may[n], succ[n] = direct, targets
		for _, t := range targets {
			if !seen[t] {
				seen[t] = true
				stack = append(stack, t)
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range nodes {
			for _, t := range succ[n] {
				if !may[n] && may[t] {
					may[n], changed = true, true
				}
			}
		}
	}
	for _, n := range nodes {
		memo[n] = may[n]
	}
	return may[ent]
}

// spawnEdges is whether ent itself takes a Future or Task (or is not a body sem summarizes), and what it calls.
func (c *borrowCheck) spawnEdges(ent sem.EntityID) (bool, []sem.EntityID) {
	var edges []sem.EffectEdge
	switch c.p.R.Entity(ent).Kind {
	case sem.EntClosure:
		edges = c.p.R.Closure(ent).Edges
	case sem.EntFn:
		edges = c.p.R.Fn(ent).Edges
	default:
		return true, nil
	}
	var out []sem.EntityID
	for _, e := range edges {
		if (e.Kind == sem.EdgeCall || e.Kind == sem.EdgeKnownCall) && e.Target != 0 {
			out = append(out, e.Target)
		}
	}
	return c.takesAsync(ent), out
}

func (c *borrowCheck) takesAsync(ent sem.EntityID) bool {
	if c.p.R.Entity(ent).Kind != sem.EntFn {
		return false
	}
	info := c.p.R.Fn(ent)
	if info.SelfParam != 0 && c.typeRuns(c.p.R.Entity(info.SelfParam).Type, runsSpawn) {
		return true
	}
	for _, prm := range info.Params {
		if c.typeRuns(c.p.R.Entity(prm).Type, runsSpawn) {
			return true
		}
	}
	return false
}
