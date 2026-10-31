package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// bodyRole is how bodyFacts passes an instruction's operand facts on.
type bodyRole uint8

const (
	// roleFresh: the result holds nothing an operand held.
	roleFresh bodyRole = iota
	// roleCarry: the result holds what the operands hold.
	roleCarry
	// roleStore: the place the first operand names takes what the last operand holds.
	roleStore
	// roleCall: the result, and every `&mut` operand or write-back cell, takes what all operands hold.
	roleCall
	// roleVoid: no result and no write a callable can reach.
	roleVoid
)

var bodyRoles = map[Opcode]bodyRole{
	OpConst: roleFresh, OpUnit: roleFresh, OpBinary: roleFresh, OpUnary: roleFresh, OpTag: roleFresh, OpIsVariant: roleFresh,
	OpIsType: roleFresh, OpInterp: roleFresh, OpShell: roleFresh, OpCancelled: roleFresh, OpCFnPtr: roleFresh,
	OpFnItem: roleFresh, OpGlobalGet: roleFresh,
	OpCopy: roleCarry, OpMove: roleCarry, OpRecord: roleCarry, OpVariant: roleCarry, OpField: roleCarry, OpFieldMove: roleCarry,
	OpArray: roleCarry, OpIndex: roleCarry, OpPayload: roleCarry, OpUnbox: roleCarry, OpBox: roleCarry, OpClosure: roleCarry,
	OpBind: roleCarry, OpBorrow: roleCarry, OpNewCell: roleCarry, OpCellGet: roleCarry, OpCellTake: roleCarry, OpAlias: roleCarry,
	OpShare: roleCarry, OpBoxDirect: roleCarry, OpUnboxDirect: roleCarry, OpLendBox: roleCarry,
	OpSetField: roleStore, OpSetIndex: roleStore, OpCellSet: roleStore, OpGlobalSet: roleStore,
	OpCall: roleCall, OpCallValue: roleCall, OpCallC: roleCall, OpBuiltin: roleCall, OpAwait: roleCall, OpAsm: roleCall, OpFutureNew: roleCall,
	OpDrop: roleVoid, OpRelease: roleVoid, OpPanic: roleVoid, OpNop: roleVoid, OpFutureDone: roleVoid, OpCancelDone: roleVoid,
}

// inertOp reports an opcode that hands none of its operands on: its result is fresh, or it has none.
func inertOp(op Opcode) bool {
	r, ok := bodyRoles[op]
	return ok && (r == roleFresh || r == roleVoid)
}

// bodyFacts says, flow-insensitively, what each local of g may hold: g's declared parameters and the fn items and
// closure literals it may be made from, by entity, and opaqueFact for a value from anywhere else.
func (c *borrowCheck) bodyFacts(g *Func) flowSets {
	fs, _ := c.bodyFlow(g)
	return fs
}

// bodyFlow is bodyFacts with kept: what stores and calls put into each local, without what it starts with; a body
// asked for again while it is being computed (a recursion) answers nil.
func (c *borrowCheck) bodyFlow(g *Func) (flowSets, flowSets) {
	m := c.offloads()
	if fs, ok := m.body[g]; ok || m.busy[g] {
		return fs, m.kept[g]
	}
	m.busy[g] = true
	defer delete(m.busy, g)
	fs, kept, escaped := flowSets{}, flowSets{}, map[uint32]bool{}
	add := func(s flowSets, l LocalID, f uint32) bool {
		if s[l][f] {
			return false
		}
		if s[l] == nil {
			s[l] = map[uint32]bool{}
		}
		s[l][f] = true
		return true
	}
	declared := c.declaredParams(g.Ent)
	for _, l := range g.Params {
		if e := g.Locals[l].Ent; e != 0 && slices.Contains(declared, e) {
			add(fs, l, uint32(e))
		} else {
			add(fs, l, opaqueFact)
		}
	}
	defs := allDefs(g)
	for changed := true; changed; {
		changed = false
		put := func(l LocalID, f uint32, store bool) {
			changed = add(fs, l, f) || changed
			if store {
				add(kept, l, f)
			}
		}
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				c.bodyStep(g, defs, in, fs, escaped, put)
			}
		}
		for f := range escaped {
			for l := range g.Locals {
				if t := g.Locals[l].Type; c.typeRuns(t, runsCallable) || c.typeRuns(t, runsHandle) {
					put(LocalID(l), f, true)
				}
			}
		}
	}
	m.body[g], m.kept[g] = fs, kept
	return fs, kept
}

// bodyStep applies in's role; a store whose place is not pinned to locals here escapes to every local able to hold it.
func (c *borrowCheck) bodyStep(g *Func, defs map[LocalID][]*Inst, in Inst, fs flowSets, escaped map[uint32]bool, put func(LocalID, uint32, bool)) {
	var from []uint32
	for _, a := range in.Args {
		for f := range fs[a] {
			from = append(from, f)
		}
	}
	into := func(roots []LocalID, ok bool, facts []uint32) {
		for _, f := range facts {
			if !ok {
				escaped[f] = true
			}
			for _, l := range roots {
				put(l, f, true)
			}
		}
	}
	for _, f := range c.bodySeeds(g, in) {
		put(in.Dst, f, false)
	}
	switch bodyRoles[in.Op] {
	case roleCarry:
		for _, f := range from {
			put(in.Dst, f, false)
		}
		if in.Op == OpClosure {
			c.captureStores(g, defs, in, into)
		}
	case roleStore:
		var stored []uint32
		for f := range fs[in.Args[len(in.Args)-1]] {
			stored = append(stored, f)
		}
		if in.Op == OpGlobalSet {
			into(nil, false, stored)
			break
		}
		roots, ok := c.placeRoots(g, defs, in.Args[0])
		into(roots, ok, stored)
	case roleCall:
		for _, f := range from {
			put(in.Dst, f, false)
		}
		for _, a := range in.Args {
			if c.lendsPlace(g, a) {
				roots, ok := c.placeRoots(g, defs, a)
				into(roots, ok, from)
			}
		}
		c.bodyCallStores(g, defs, in, fs, into, put)
	}
}

// captureStores merges into each local closure in captures what the closure's body may store into that capture; a
// parameter of the closure stored there is a value from elsewhere here.
func (c *borrowCheck) captureStores(g *Func, defs map[LocalID][]*Inst, in Inst, into func([]LocalID, bool, []uint32)) {
	if c.p.R.Entity(in.Ent).Kind != sem.EntClosure {
		return
	}
	h := c.p.ByEnt[in.Ent]
	_, kept, ok := c.finishedFlow(h)
	for j, a := range in.Args {
		var facts []uint32
		if !ok || j >= len(h.Params) {
			facts = []uint32{opaqueFact}
		} else {
			for f := range kept[h.Params[j]] {
				if f != opaqueFact && c.p.R.Entity(sem.EntityID(f)).Kind == sem.EntParam {
					f = opaqueFact
				}
				facts = append(facts, f)
			}
		}
		roots, ok := c.placeRoots(g, defs, a)
		into(roots, ok, facts)
	}
}

// bodySeeds are the facts in itself gives its result: a fn item, bound fn or closure literal, and opaqueFact for a
// global or a callable a call hands back by value, which may be one its callee made.
func (c *borrowCheck) bodySeeds(g *Func, in Inst) []uint32 {
	switch {
	case in.Op == OpFnItem, in.Op == OpBind, in.Op == OpClosure && c.p.R.Entity(in.Ent).Kind == sem.EntClosure:
		return []uint32{uint32(in.Ent)}
	case in.Op == OpGlobalGet:
		return []uint32{opaqueFact}
	case bodyRoles[in.Op] == roleCall && in.Op.Yields() && c.p.R.Types.Kind(g.Locals[in.Dst].Type) != sem.KRef &&
		c.typeRuns(g.Locals[in.Dst].Type, runsCallable):
		return []uint32{opaqueFact}
	}
	return nil
}
