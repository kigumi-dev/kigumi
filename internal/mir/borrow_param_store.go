package mir

import (
	"slices"

	"kigumi/internal/sem"
)

type paramKey struct {
	g *Func
	i int
}

// derives lists the opcodes whose result reaches what an operand points at: copies, cells, borrows and projections.
var derives = map[Opcode]bool{OpCopy: true, OpMove: true, OpAlias: true, OpShare: true, OpNewCell: true, OpCellGet: true, OpCellTake: true,
	OpBorrow: true, OpField: true, OpFieldMove: true, OpPayload: true, OpIndex: true, OpUnbox: true}

// paramStores reports whether running g may store through its parameter i, as a closure body may through a captured
// `&mut` holder: a write into a place under it, or a hand-off of a `&mut` reached from it to code that may store.
func (c *borrowCheck) paramStores(g *Func, i int) bool {
	if g == nil || i >= len(g.Params) {
		return true
	}
	key := paramKey{g, i}
	if v, ok := c.storeMemo[key]; ok {
		return v
	}
	if c.storeMemo == nil {
		c.storeMemo = map[paramKey]bool{}
	}
	c.storeMemo[key] = true
	from := map[LocalID]bool{g.Params[i]: true}
	for changed := true; changed; {
		changed = false
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if derives[in.Op] && !from[in.Dst] && slices.ContainsFunc(in.Args, func(a LocalID) bool { return from[a] }) {
					from[in.Dst], changed = true, true
				}
			}
		}
	}
	holdsMut := func(l LocalID) bool {
		return from[l] && c.typeHolds(g.Locals[l].Type, func(t sem.TypeID) bool { _, mut := c.p.R.MutRefElem(t); return mut })
	}
	v := false
	for _, blk := range g.Blocks {
		v = v || slices.ContainsFunc(blk.Term.Args, holdsMut) || slices.ContainsFunc(blk.Insts, func(in Inst) bool { return c.instStores(g, in, from, holdsMut) })
	}
	c.storeMemo[key] = v
	return v
}

// instStores reports whether in may store something a flow tracks through a value reached from the parameter: a
// write of such a value under it, or a `&mut` from it handed on to code that may store, or put anywhere else.
func (c *borrowCheck) instStores(g *Func, in Inst, from map[LocalID]bool, holdsMut func(LocalID) bool) bool {
	switch {
	case in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet:
		v := in.Args[len(in.Args)-1]
		return from[in.Args[0]] && c.carries(g.Locals[v].Type) || holdsMut(v)
	case derives[in.Op], in.Op == OpDrop, in.Op == OpRelease:
		return false
	}
	for k, a := range in.Args {
		if !holdsMut(a) {
			continue
		}
		if h := c.p.ByEnt[in.Ent]; in.Op == OpClosure || in.Op == OpCall && h != nil && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
			if c.paramStores(h, k) {
				return true
			}
			continue
		}
		// A callee without a body here can only store what it is handed, and an index cannot hold a closure.
		other := func(b LocalID) bool { return b != a && c.carries(g.Locals[b].Type) }
		if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpBuiltin || slices.ContainsFunc(in.Args, other) {
			return true
		}
	}
	return false
}

// carries reports whether a value of type t may hold what the flows track: something callable, a Future or Task, a
// borrow, a resource whose drop runs, or a type parameter.
func (c *borrowCheck) carries(t sem.TypeID) bool {
	tt := c.p.R.Types
	return c.typeRuns(t, runsDrop) || c.typeHolds(t, func(t sem.TypeID) bool {
		switch tt.Kind(t) {
		case sem.KFn, sem.KClosure, sem.KIface, sem.KRef, sem.KParam, sem.KVar:
			return true
		}
		return tt.Kind(t) == sem.KNamed && tt.Node(t).Ent == tt.FutureEnt() || c.p.R.IsTaskType(t)
	})
}
