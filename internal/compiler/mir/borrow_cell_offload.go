package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// offBit marks an offload flow fact as a closure handed to another thread; without it the fact is a closure or an
// async fn a value holds.
const offBit uint32 = 1 << 31

// opaqueFact marks a callable value of an origin this body cannot see: a call's result, or one lent `&mut` to a call.
const opaqueFact uint32 = 0

// offloadMemo keeps the call-graph answers about handing work off, which do not depend on the body asking.
type offloadMemo struct {
	gate                     int8
	may, detach, keep, calls map[sem.EntityID]bool
	body, kept               map[*Func]flowSets
	keeps                    map[keptKey]*keptSummary
	busy                     map[*Func]bool
	shared                   *sinkMemo
}

func (c *borrowCheck) offloads() *offloadMemo {
	m := &c.p.offloads
	if m.gate == 0 {
		m.gate, m.may, m.detach, m.keep, m.calls = -1, map[sem.EntityID]bool{}, map[sem.EntityID]bool{}, map[sem.EntityID]bool{}, map[sem.EntityID]bool{}
		m.body, m.kept, m.keeps, m.busy = map[*Func]flowSets{}, map[*Func]flowSets{}, map[keptKey]*keptSummary{}, map[*Func]bool{}
		for id := 1; id < len(c.p.R.Entities); id++ {
			if c.offloadPrim(sem.EntityID(id)) {
				m.gate = 1
				break
			}
		}
	}
	return m
}

// offloadPossible reports whether the program declares an offloadPrim at all.
func (c *borrowCheck) offloadPossible() bool { return c.offloads().gate > 0 }

// offloadPrim reports a declaration without a body that takes something callable and returns a Future or Task: it may
// run what it is handed on a thread of its own and return before that ends.
func (c *borrowCheck) offloadPrim(ent sem.EntityID) bool {
	if c.p.R.Entity(ent).Kind != sem.EntFn || !c.p.R.ThreadPoolAllowed() {
		return false
	}
	info := c.p.R.Fn(ent)
	if info.Body != 0 || !c.typeRuns(c.p.R.Types.Node(info.Sig).Elem, runsSpawn) {
		return false
	}
	return slices.ContainsFunc(append([]sem.EntityID{info.SelfParam}, info.Params...), func(p sem.EntityID) bool {
		return p != 0 && c.typeRuns(c.p.R.Entity(p).Type, runsCallable)
	})
}

// mayOffload reports whether running fn or closure ent may hand work to another thread: it, or something it calls
// or calls back, is an offloadPrim.
func (c *borrowCheck) mayOffload(ent sem.EntityID) bool {
	return c.offloadPossible() && c.callGraphMay(ent, c.offloads().may, func(e sem.EntityID) (bool, []sem.EntityID) {
		var out []sem.EntityID
		for _, edge := range c.entityEdges(e) {
			if (edge.Kind == sem.EdgeCall || edge.Kind == sem.EdgeKnownCall || edge.Kind == sem.EdgeCallback) && edge.Target != 0 {
				out = append(out, edge.Target)
			}
		}
		return c.offloadPrim(e), out
	})
}

func (c *borrowCheck) entityEdges(ent sem.EntityID) []sem.EffectEdge {
	switch c.p.R.Entity(ent).Kind {
	case sem.EntClosure:
		return c.p.R.Closure(ent).Edges
	case sem.EntFn:
		return c.p.R.Fn(ent).Edges
	}
	return nil
}

// offloadsAt reports whether call in may return while work it was handed still runs on another thread: its callee
// may offload, or is unknown and returns a Future or Task.
func (c *borrowCheck) offloadsAt(in Inst) bool {
	switch in.Op {
	case OpCall:
		if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
			return c.mayOffload(in.Ent)
		}
	case OpCallValue:
		if cl := c.calleeClosure(c.f, in); cl != 0 {
			return c.mayOffload(cl)
		}
	default:
		return false
	}
	return c.offloadPossible() && c.typeRuns(c.f.Locals[in.Dst].Type, runsSpawn)
}

func (c *borrowCheck) offloadSeed(in Inst) []uint32 {
	if in.Op == OpClosure && c.p.R.Entity(in.Ent).Kind == sem.EntClosure || in.Op == OpFnItem {
		return []uint32{uint32(in.Ent)}
	}
	return nil
}

// offloadStep hands an offloading call's runnable arguments to its result and lent places; what a call or await may
// detach, handed off or offloadable later, goes to the task scope and to the result, so awaiting that Task is no rerun.
func (c *borrowCheck) offloadStep(in Inst, s flowSets, put func(LocalID, uint32)) {
	c.captureStep(in, s, put)
	c.opaqueStep(in, s, put)
	if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpAwait && in.Op != OpFutureNew {
		return
	}
	off := (in.Op == OpCall || in.Op == OpCallValue) && c.offloadsAt(in)
	fwd := in.Op != OpAwait && c.forwardDetaches(in, s)
	for k, a := range in.Args {
		held, handed := c.splitOffloadFacts(s[a], a)
		hands := off && c.argRunnable(in, k)
		if hands {
			for _, d := range c.offloadDsts(in, s) {
				c.handOff(in, held, d, put)
			}
		}
		byCallee := in.Op != OpFutureNew && c.argDetaches(in, k, held)
		if !byCallee && !(fwd && (in.Op == OpFutureNew || c.slotKeepsHandle(c.f, in, k))) {
			continue
		}
		for _, f := range handed {
			put(c.spawnScope(), f)
		}
		if hands || slices.ContainsFunc(held, c.mayOffload) {
			c.handOff(in, held, c.spawnScope(), put)
			c.handOff(in, held, in.Dst, put)
		}
	}
}

func (c *borrowCheck) handOff(in Inst, held []sem.EntityID, d LocalID, put func(LocalID, uint32)) {
	for _, cl := range held {
		if c.p.R.Entity(cl).Kind != sem.EntClosure {
			continue
		}
		put(d, uint32(cl)|offBit)
		if _, ok := c.offloadNode[cl]; !ok {
			c.offloadNode[cl] = in.Node
		}
	}
}

// splitOffloadFacts parts what a value carries into the closures and async fns it holds and the closures it handed off.
func (c *borrowCheck) splitOffloadFacts(facts map[uint32]bool, a LocalID) (held []sem.EntityID, handed []uint32) {
	for f := range facts {
		switch {
		case f&offBit != 0:
			handed = append(handed, f)
		case f != opaqueFact:
			held = append(held, sem.EntityID(f))
		}
	}
	for _, cl := range c.typeClosures(c.f.Locals[a].Type) {
		if !slices.Contains(held, cl) {
			held = append(held, cl)
		}
	}
	return held, handed
}

// offloadDsts are where an offloading call may leave the handle of the work it hands off: its result, what it lends
// `&mut`, and the shared storage it may store into.
func (c *borrowCheck) offloadDsts(in Inst, s flowSets) []LocalID {
	dsts := []LocalID{in.Dst}
	for _, a := range in.Args {
		for _, p := range c.lentMutPlaces(a) {
			dsts = append(append(dsts, p.locals()...), c.heldMutOwners(p.root)...)
		}
	}
	for _, k := range c.keptArgs(in, s) {
		dsts = append(dsts, c.storePlaces(in.Args[k])...)
	}
	return dsts
}
