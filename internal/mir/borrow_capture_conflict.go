package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// changes lists in-place writes and field moves; whole reassignment is left out since the closure keeps the old value.
func (c *readCheck) changes() []moveEvent {
	var out []moveEvent
	for bi := range c.f.Blocks {
		for i, in := range c.f.Blocks[bi].Insts {
			changed, field := false, false
			switch in.Op {
			case OpSetField, OpSetIndex:
				changed = true
			case OpFieldMove:
				changed, field = true, true
			case OpBorrow:
				changed = in.Index == 1
			}
			if r := c.root(firstArg(in)); changed && r != 0 && c.f.Locals[r].Ent != 0 {
				out = append(out, moveEvent{at: movePos{BlockID(bi), i}, root: r, node: in.Node, field: field})
			}
			if r := c.cellTaken(in); r != 0 {
				out = append(out, moveEvent{at: movePos{BlockID(bi), i}, root: r, node: in.Node})
			}
		}
	}
	return out
}

func (c *readCheck) cellTaken(in Inst) LocalID {
	for i, a := range in.Args {
		d := c.defs[a]
		if d == nil || d.Op != OpCellGet || c.f.Locals[d.Args[0]].Ent == 0 {
			continue
		}
		if in.Op == OpMove || constructs(in.Op) || c.p.consumesArg(&in, i) || c.p.primitiveTakes(&in, i) {
			return d.Args[0]
		}
	}
	return 0
}

func firstArg(in Inst) LocalID {
	if len(in.Args) == 0 {
		return 0
	}
	return in.Args[0]
}

func (c *readCheck) escapes(in Inst, caps map[LocalID][]capturedPlace) []LocalID {
	var out []LocalID
	for i, a := range in.Args {
		if _, ok := caps[a]; ok && c.escapesAt(in, i) {
			out = append(out, a)
		}
	}
	return out
}

func (c *readCheck) escapesAt(in Inst, i int) bool {
	switch in.Op {
	case OpCall:
		if c.p.R.Entity(in.Ent).Kind != sem.EntFn {
			return true
		}
		info := c.p.R.Fn(in.Ent)
		idx := i
		if info.Recv != sem.RecvNone {
			if i == 0 {
				return false
			}
			idx--
		}
		return idx >= len(info.Params) || !c.p.R.NonEscapingParam(info.Params[idx]) && !c.lentParam(info.Params[idx])
	case OpCallValue:
		return i != 0
	case OpSetField, OpSetIndex, OpCellSet, OpGlobalSet, OpNewCell, OpCallC, OpBuiltin, OpAsm, OpShell:
		return true
	}
	return false
}

// lentParam reports a reference parameter without a lifetime, which the callee cannot keep past the call.
func (c *readCheck) lentParam(prm sem.EntityID) bool {
	t := c.p.R.Entity(prm).Type
	if c.p.R.Types.Kind(t) != sem.KRef || c.p.R.Local(prm).Lifetime != 0 {
		return false
	}
	return !slices.ContainsFunc(c.p.R.Types.Params(t), func(p sem.EntityID) bool { return c.p.R.TypeParam(p).IsLifetime })
}

func (c *readCheck) root(l LocalID) LocalID {
	for d := c.defs[l]; d != nil; d = c.defs[l] {
		switch {
		case d.Op == OpField || d.Op == OpIndex || d.Op == OpPayload:
		case d.Op == OpCopy && c.f.Locals[l].Ent == 0:
		default:
			return l
		}
		l = d.Args[0]
	}
	return l
}

func soleDefs(f *Func) map[LocalID]*Inst {
	defs := map[LocalID]*Inst{}
	multi := map[LocalID]bool{}
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Insts {
			in := &f.Blocks[bi].Insts[ii]
			if !in.Op.Yields() {
				continue
			}
			if defs[in.Dst] != nil {
				multi[in.Dst] = true
			}
			defs[in.Dst] = in
		}
	}
	for l := range multi {
		delete(defs, l)
	}
	return defs
}
