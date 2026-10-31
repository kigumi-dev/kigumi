package borrow

import (
	"maps"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// allDefs lists every instruction that defines each local of g.
func allDefs(g *midir.Func) map[midir.LocalID][]*midir.Inst {
	out := map[midir.LocalID][]*midir.Inst{}
	for bi := range g.Blocks {
		for ii := range g.Blocks[bi].Insts {
			if in := &g.Blocks[bi].Insts[ii]; in.Op.Yields() && definesDst(*in) {
				out[in.Dst] = append(out[in.Dst], in)
			}
		}
	}
	return out
}

// placeRoots are the locals a write through l may reach, following every definition back through projections, borrows,
// cells, copies and, for a returned reference, the call's operands; not ok when a reference comes from anywhere else.
func (c *borrowCheck) placeRoots(g *midir.Func, defs map[midir.LocalID][]*midir.Inst, l midir.LocalID) ([]midir.LocalID, bool) {
	out, ok := []midir.LocalID{l}, true
	for i := 0; i < len(out); i++ {
		x := out[i]
		isRef := c.p.R.Types.Kind(g.Locals[x].Type) == sem.KRef
		for _, d := range defs[x] {
			var next []midir.LocalID
			switch {
			case len(d.Args) == 0:
				ok = ok && !isRef
			case bodyRoles[d.Op] == roleCarry:
				next = d.Args[:1]
				switch d.Op {
				case midir.OpCopy, midir.OpMove, midir.OpAlias, midir.OpShare, midir.OpRecord, midir.OpVariant, midir.OpArray:
					next = d.Args
				}
			case bodyRoles[d.Op] == roleCall:
				next = d.Args
			default:
				ok = ok && !isRef
			}
			for _, n := range next {
				if !slices.Contains(out, n) {
					out = append(out, n)
				}
			}
		}
	}
	return out, ok
}

// calleeOf sorts what call in of g may run: fn items and closures by entity, g's own declared parameters, and whether
// a callee from anywhere else is possible.
func (c *borrowCheck) calleeOf(g *midir.Func, fs flowSets, in midir.Inst) (ents, params []sem.EntityID, unseen bool) {
	switch {
	case in.Op == midir.OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn:
		return []sem.EntityID{in.Ent}, nil, false
	case in.Op == midir.OpCall:
		return nil, nil, true
	case in.Op == midir.OpCallValue && c.calleeClosure(g, in) != 0:
		return []sem.EntityID{c.calleeClosure(g, in)}, nil, false
	case in.Op != midir.OpCallValue && in.Op != midir.OpFutureNew:
		return nil, nil, false
	}
	held := fs[in.Args[0]]
	unseen = !slices.ContainsFunc(slices.Collect(maps.Keys(held)), func(f uint32) bool { return f&offBit == 0 })
	for f := range held {
		switch e := sem.EntityID(f); {
		case f&offBit != 0:
		case f == opaqueFact:
			unseen = true
		case c.p.R.Entity(e).Kind == sem.EntFn || c.p.R.Entity(e).Kind == sem.EntClosure:
			ents = append(ents, e)
		case c.p.R.Entity(e).Kind == sem.EntParam:
			params = append(params, e)
		default:
			unseen = true
		}
	}
	slices.Sort(ents)
	return ents, params, unseen
}

// boundParams are the parameters of the given callees that argument k of in binds: a value call's callee is no
// argument of its own, except a method value's, which is its receiver.
func (c *borrowCheck) boundParams(ents []sem.EntityID, in midir.Inst, k int) []sem.EntityID {
	var out []sem.EntityID
	for _, e := range ents {
		j := k
		if in.Op != midir.OpCall && c.methodOf(e) == 0 {
			j--
		}
		if params := c.declaredParams(e); j >= 0 && j < len(params) {
			out = append(out, params[j])
		}
	}
	return out
}
