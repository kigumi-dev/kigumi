package mir

import (
	"maps"
	"slices"

	"kigumi/internal/sem"
)

// allDefs lists every instruction that defines each local of g.
func allDefs(g *Func) map[LocalID][]*Inst {
	out := map[LocalID][]*Inst{}
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
func (c *borrowCheck) placeRoots(g *Func, defs map[LocalID][]*Inst, l LocalID) ([]LocalID, bool) {
	out, ok := []LocalID{l}, true
	for i := 0; i < len(out); i++ {
		x := out[i]
		isRef := c.p.R.Types.Kind(g.Locals[x].Type) == sem.KRef
		for _, d := range defs[x] {
			var next []LocalID
			switch {
			case len(d.Args) == 0:
				ok = ok && !isRef
			case bodyRoles[d.Op] == roleCarry:
				next = d.Args[:1]
				if d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias || d.Op == OpShare {
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
func (c *borrowCheck) calleeOf(g *Func, fs flowSets, in Inst) (ents, params []sem.EntityID, unseen bool) {
	switch {
	case in.Op == OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn:
		return []sem.EntityID{in.Ent}, nil, false
	case in.Op == OpCall:
		return nil, nil, true
	case in.Op == OpCallValue && c.calleeClosure(g, in) != 0:
		return []sem.EntityID{c.calleeClosure(g, in)}, nil, false
	case in.Op != OpCallValue && in.Op != OpFutureNew:
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

// boundParams are the parameters of the given callees that argument k of in binds.
func (c *borrowCheck) boundParams(ents []sem.EntityID, in Inst, k int) []sem.EntityID {
	if in.Op != OpCall {
		k--
	}
	var out []sem.EntityID
	for _, e := range ents {
		if params := c.declaredParams(e); k >= 0 && k < len(params) {
			out = append(out, params[k])
		}
	}
	return out
}
