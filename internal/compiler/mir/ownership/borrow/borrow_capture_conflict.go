package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// changes lists in-place writes and field moves; whole reassignment is left out since the closure keeps the old value.
func (c *readCheck) changes() []ownquery.MoveEvent {
	var out []ownquery.MoveEvent
	for bi := range c.f.Blocks {
		for i, in := range c.f.Blocks[bi].Insts {
			changed, field := false, false
			switch in.Op {
			case midir.OpSetField, midir.OpSetIndex:
				changed = true
			case midir.OpFieldMove:
				changed, field = true, true
			case midir.OpBorrow:
				changed = in.Index == 1
			}
			if changed {
				if r := c.root(in.Args[0]); c.f.Locals[r].Ent != 0 {
					out = append(out, ownquery.MoveEvent{At: ownquery.MovePosition{Bid: midir.BlockID(bi), Idx: i}, Root: r, Node: in.Node, Field: field})
				}
			}
			if r, ok := c.cellTaken(in); ok {
				out = append(out, ownquery.MoveEvent{At: ownquery.MovePosition{Bid: midir.BlockID(bi), Idx: i}, Root: r, Node: in.Node})
			}
		}
	}
	return out
}

func (c *readCheck) cellTaken(in midir.Inst) (midir.LocalID, bool) {
	for i, a := range in.Args {
		d := c.defs[a]
		if d == nil || d.Op != midir.OpCellGet || c.f.Locals[d.Args[0]].Ent == 0 {
			continue
		}
		if in.Op == midir.OpMove || ownquery.Constructs(in.Op) || ownquery.ConsumesArg(c.p.R, c.p.ByEnt, &in, i) || ownquery.PrimitiveTakes(c.p.R, &in, i) {
			return d.Args[0], true
		}
	}
	return 0, false
}

func (c *readCheck) escapes(in midir.Inst, caps map[midir.LocalID][]capturedPlace) []midir.LocalID {
	var out []midir.LocalID
	for i, a := range in.Args {
		if _, ok := caps[a]; ok && c.escapesAt(in, i) {
			out = append(out, a)
		}
	}
	return out
}

func (c *readCheck) escapesAt(in midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpCall:
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
	case midir.OpCallValue:
		return i != 0
	case midir.OpSetField, midir.OpSetIndex, midir.OpCellSet, midir.OpGlobalSet, midir.OpNewCell, midir.OpCallC, midir.OpBuiltin, midir.OpAsm, midir.OpShell:
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

func (c *readCheck) root(l midir.LocalID) midir.LocalID {
	for d := c.defs[l]; d != nil; d = c.defs[l] {
		switch {
		case d.Op == midir.OpField || d.Op == midir.OpIndex || d.Op == midir.OpPayload:
		case d.Op == midir.OpCopy && c.f.Locals[l].Ent == 0:
		default:
			return l
		}
		l = d.Args[0]
	}
	return l
}

func (c *borrowCheck) soleDef(l midir.LocalID) *midir.Inst { return c.soleDefMap()[l] }

func (c *borrowCheck) soleDefMap() map[midir.LocalID]*midir.Inst {
	if c.sole == nil {
		c.sole = mirflow.SoleDefs(c.f)
	}
	return c.sole
}
