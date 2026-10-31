package resource

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// trustedStdFrame is the stack contribution trusted for a resolved
// bodiless std operation: a small constant, not a measured native frame.
const trustedStdFrame = 2

// Compute estimates frames and the worst-case static call chain for each body.
func Compute(r *sem.Result, funcs []*midir.Func, bodies map[sem.EntityID]*midir.Func) map[sem.EntityID]*Summary {
	word := uint64(r.Types.PtrBits()) / 8
	if word == 0 {
		word = 8
	}
	cs := &chainState{
		r: r, bodies: bodies,
		word:    word,
		frame:   map[sem.EntityID]uint64{},
		bounds:  map[sem.EntityID]StackBound{},
		onStack: map[sem.EntityID]bool{},
	}
	for _, f := range funcs {
		cs.frame[f.Ent] = frameBytes(r, f, word)
	}
	static := staticWritableBytes(r, word)
	out := map[sem.EntityID]*Summary{}
	for _, f := range funcs {
		out[f.Ent] = &Summary{
			FrameBytes: cs.frame[f.Ent],
			Chain:      cs.chain(f.Ent),
			Async:      asyncBound(r, f),
			Static:     static,
		}
	}
	return out
}

type chainState struct {
	r       *sem.Result
	bodies  map[sem.EntityID]*midir.Func
	word    uint64
	frame   map[sem.EntityID]uint64
	bounds  map[sem.EntityID]StackBound
	onStack map[sem.EntityID]bool
}

// chain is a memoized DFS with on-stack cycle detection: finding a node
// still being visited is a cycle, unbounded by definition.
func (cs *chainState) chain(ent sem.EntityID) StackBound {
	if b, ok := cs.bounds[ent]; ok {
		return b
	}
	own := cs.frame[ent]
	if cs.onStack[ent] {
		// Path is empty here: the caller's combineBound already prepends
		// this same name as edgeName, so a non-empty Path would double it.
		name := cs.r.Entity(ent).Name
		return StackBound{Kind: Unbounded, Reason: "`" + name + "` calls back into its own call chain"}
	}
	f := cs.bodies[ent]
	if f == nil {
		return StackBound{Kind: Bounded, Bytes: own}
	}
	cs.onStack[ent] = true
	acc := StackBound{Kind: Bounded, Bytes: own}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op != midir.OpCall && inst.Op != midir.OpCallValue {
				continue
			}
			name, edge := cs.edgeBound(f, inst)
			acc = combineBound(acc, own, edge, name)
		}
	}
	delete(cs.onStack, ent)
	cs.bounds[ent] = acc
	return acc
}

// A std primitive is trusted as a constant unless it routes through a known callback.
func (cs *chainState) edgeBound(f *midir.Func, inst midir.Inst) (string, StackBound) {
	if id, ok := inst.StdOp(); ok {
		if argIdx, isHO := opdef.CallbackArg(id); isHO {
			return cs.callbackEdgeBound(f, inst, argIdx)
		}
		return "<std>", StackBound{Kind: Bounded, Bytes: trustedStdFrame * cs.word}
	}
	if inst.Ent == 0 {
		return "<value>", StackBound{Kind: Unknown, Reason: "calls through a function value whose target this build cannot resolve statically"}
	}
	name := cs.r.Entity(inst.Ent).Name
	if cs.bodies[inst.Ent] == nil {
		if trustedLeaf(cs.r, inst.Ent) {
			return name, StackBound{Kind: Bounded, Bytes: trustedStdFrame * cs.word}
		}
		reason := "calls `" + name + "`, which has no body this build can measure (an `extern` declaration, or an interface/witness dispatch not resolved to one implementation)"
		return name, StackBound{Kind: Unknown, Reason: reason}
	}
	return name, cs.chain(inst.Ent)
}

// callbackEdgeBound chains into a callback target resolved to exactly one
// known function/closure literal; a stored or borrowed one stays Unknown.
func (cs *chainState) callbackEdgeBound(f *midir.Func, inst midir.Inst, argIdx int) (string, StackBound) {
	primName := cs.r.Entity(inst.Ent).Name
	if argIdx >= len(inst.Args) {
		return primName, StackBound{Kind: Bounded, Bytes: trustedStdFrame * cs.word}
	}
	ent, ok := calleeConst(f, inst.Args[argIdx])
	if !ok {
		reason := "calls `" + primName + "`, whose callback argument this build cannot resolve to a fixed target"
		return primName, StackBound{Kind: Unknown, Reason: reason}
	}
	if cs.bodies[ent] == nil {
		reason := "calls `" + primName + "`'s callback, which has no body this build can measure"
		return primName, StackBound{Kind: Unknown, Reason: reason}
	}
	return primName, cs.chain(ent)
}

// calleeConst resolves local through plan-made copies; any other or repeated write is unresolvable.
func calleeConst(f *midir.Func, local midir.LocalID) (sem.EntityID, bool) {
	for range f.Locals {
		var def *midir.Inst
		for bid := range f.Blocks {
			for i := range f.Blocks[bid].Insts {
				inst := &f.Blocks[bid].Insts[i]
				if !inst.Op.Yields() || inst.Dst != local {
					continue
				}
				if def != nil {
					return 0, false
				}
				def = inst
			}
		}
		switch {
		case def == nil:
			return 0, false
		case def.Op == midir.OpClosure || def.Op == midir.OpFnItem:
			return def.Ent, true
		case def.Op == midir.OpCopy || def.Op == midir.OpAlias || def.Op == midir.OpShare || def.Op == midir.OpMove || def.Op == midir.OpBoxDirect || def.Op == midir.OpUnboxDirect:
			local = def.Args[0]
		default:
			return 0, false
		}
	}
	return 0, false
}

// combineBound folds one edge into the running worst-case bound: Unbounded
// beats Unknown beats a larger Bounded byte count; the first cause wins.
func combineBound(acc StackBound, own uint64, edge StackBound, edgeName string) StackBound {
	switch edge.Kind {
	case Unbounded:
		if acc.Kind == Unbounded {
			return acc
		}
		return StackBound{Kind: Unbounded, Reason: edge.Reason, Path: append([]string{edgeName}, edge.Path...)}
	case Unknown:
		if acc.Kind != Bounded {
			return acc
		}
		return StackBound{Kind: Unknown, Reason: edge.Reason, Path: append([]string{edgeName}, edge.Path...)}
	default:
		if acc.Kind != Bounded {
			return acc
		}
		if total := own + edge.Bytes; total > acc.Bytes {
			return StackBound{Kind: Bounded, Bytes: total}
		}
		return acc
	}
}
