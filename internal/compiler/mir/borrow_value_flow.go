package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// valueFlow follows seeded facts through copies, fn coercions, futures, containers, stores, lent `&mut` places and step:
// in is each block's entry state, first where a block adds a fact, ever every fact a local takes anywhere.
type valueFlow struct {
	seed  func(midir.Inst) []uint32
	step  flowStep
	in    []flowSets
	first map[midir.BlockID]map[midir.LocalID]map[uint32]int
	ever  flowSets
}

// flowStep adds the facts in passes on beyond its own result, given the state just after it.
type flowStep func(in midir.Inst, s flowSets, put func(midir.LocalID, uint32))

// valueFlowFresh lists the opcodes whose result cannot carry a value from an operand.
var valueFlowFresh = map[midir.Opcode]bool{midir.OpConst: true, midir.OpUnit: true, midir.OpBinary: true, midir.OpUnary: true, midir.OpTag: true, midir.OpIsVariant: true,
	midir.OpIsType: true, midir.OpInterp: true, midir.OpShell: true, midir.OpFnItem: true, midir.OpCFnPtr: true, midir.OpGlobalGet: true, midir.OpCancelled: true}

// spawnScope stands for the running task's scope, which keeps every task an argument may have spawned until the
// body ends; it is one past the body's locals.
func (c *borrowCheck) spawnScope() midir.LocalID { return midir.LocalID(len(c.f.Locals)) }

func (c *borrowCheck) solveFlow(seed func(midir.Inst) []uint32, step flowStep) *valueFlow {
	n := len(c.f.Blocks)
	fl := &valueFlow{seed: seed, step: step, in: make([]flowSets, n), first: map[midir.BlockID]map[midir.LocalID]map[uint32]int{}, ever: flowSets{}}
	for i := range fl.in {
		fl.in[i] = flowSets{}
	}
	keep := c.crossBlockLocals()
	order := reversePostorder(c.f)
	dirty := make([]bool, n)
	for _, b := range order {
		dirty[b] = true
	}
	for changed := true; changed; {
		changed = false
		for _, bid := range order {
			if !dirty[bid] {
				continue
			}
			dirty[bid] = false
			s := c.replayFlow(fl, cloneFlowSets(fl.in[bid]), bid, nil)
			for l := range s {
				if !keep[l] {
					delete(s, l)
				}
			}
			for _, t := range c.f.Blocks[bid].Term.Targets {
				if mergeFlowSets(fl.in[t], s) {
					dirty[t], changed = true, true
				}
			}
		}
	}
	for bid := range c.f.Blocks {
		mergeFlowSets(fl.ever, c.replayFlow(fl, cloneFlowSets(fl.in[bid]), midir.BlockID(bid), nil))
	}
	return fl
}

// flowFirst is where each fact first enters a local in the current block.
func (c *borrowCheck) flowFirst(fl *valueFlow) map[midir.LocalID]map[uint32]int {
	if fl.first[c.curBlock] == nil {
		first := map[midir.LocalID]map[uint32]int{}
		fl.first[c.curBlock] = first
		c.replayFlow(fl, cloneFlowSets(fl.in[c.curBlock]), c.curBlock, func(l midir.LocalID, f uint32, i int) {
			if first[l] == nil {
				first[l] = map[uint32]int{}
			}
			first[l][f] = i
		})
	}
	return fl.first[c.curBlock]
}

// flowHas reports whether l may carry f just before the instruction being checked.
func (c *borrowCheck) flowHas(fl *valueFlow, l midir.LocalID, f uint32) bool {
	if fl.in[c.curBlock][l][f] {
		return true
	}
	i, ok := c.flowFirst(fl)[l][f]
	return ok && i < c.curIdx
}

// flowAt is what l may carry just before the instruction being checked.
func (c *borrowCheck) flowAt(fl *valueFlow, l midir.LocalID) map[uint32]bool {
	c.flowFirst(fl)
	out := map[uint32]bool{}
	for f := range fl.in[c.curBlock][l] {
		out[f] = true
	}
	for f, i := range fl.first[c.curBlock][l] {
		if i < c.curIdx {
			out[f] = true
		}
	}
	return out
}

// replayFlow applies a block to s, calling added for each fact an instruction adds.
func (c *borrowCheck) replayFlow(fl *valueFlow, s flowSets, bid midir.BlockID, added func(midir.LocalID, uint32, int)) flowSets {
	put := func(l midir.LocalID, f uint32, i int) {
		if s[l][f] || l != c.spawnScope() && c.p.retainOnly(c.f.Locals[l].Type) {
			return
		}
		if s[l] == nil {
			s[l] = map[uint32]bool{}
		}
		s[l][f] = true
		if added != nil {
			added(l, f, i)
		}
	}
	for i, in := range c.f.Blocks[bid].Insts {
		for _, f := range fl.seed(in) {
			if in.Op.Yields() {
				put(in.Dst, f, i)
			}
		}
		dsts, srcs := c.flowEdges(in)
		for _, d := range dsts {
			for _, a := range srcs {
				for f := range s[a] {
					put(d, f, i)
				}
			}
		}
		if fl.step != nil {
			fl.step(in, s, func(l midir.LocalID, f uint32) { put(l, f, i) })
		}
	}
	return s
}

// flowEdges says where in's operands flow: its result, the place a store writes, and for a call or a closure also
// the places it lends `&mut` (captured holders its body may store through), where what it was handed may be kept.
func (c *borrowCheck) flowEdges(in midir.Inst) (dsts, srcs []midir.LocalID) {
	switch {
	case in.Op == midir.OpSetField || in.Op == midir.OpSetIndex || in.Op == midir.OpCellSet:
		if root, _, _, ok := c.projectionPlace(in.Args[0]); ok {
			dsts = append(dsts, root)
		}
		for _, p := range c.refPlaces(in.Args[0]) {
			dsts = append(dsts, p.root)
		}
		return dsts, in.Args[len(in.Args)-1:]
	case in.Op == midir.OpCall || in.Op == midir.OpCallValue || in.Op == midir.OpFutureNew || in.Op == midir.OpBuiltin || in.Op == midir.OpClosure:
		dsts = append(dsts, in.Dst)
		for k, a := range in.Args {
			if in.Op == midir.OpClosure && !c.paramStores(c.p.ByEnt[in.Ent], k) {
				continue
			}
			for _, p := range c.lentMutPlaces(a) {
				dsts = append(append(dsts, p.locals()...), c.heldMutOwners(p.root)...)
			}
		}
		return dsts, in.Args
	case !in.Op.Yields() || valueFlowFresh[in.Op]:
		return nil, nil
	}
	return []midir.LocalID{in.Dst}, in.Args
}
