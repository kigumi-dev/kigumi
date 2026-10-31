package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// An unbox step leaves a readHop inexact.
type readHop struct {
	path  stepPath
	exact bool
}

// Only a holder that keeps its literal's fields answers per field; any other gives every owner it holds.
func (c *borrowCheck) heldAt(o ownerRef, all []ownerRef, hop readHop) []ownerRef {
	path := o.path.with(hop.path...)
	if !hop.exact || len(o.path) > 0 && !o.exact || len(path) == 0 || !c.fieldStable(o.owner) {
		return all
	}
	if out := c.partHeld(c.literalOf(o.owner), path); len(out) > 0 {
		return out
	}
	return all
}

// partHeld returns nil when a step does not fit lit.
func (c *borrowCheck) partHeld(lit *midir.Inst, path stepPath) []ownerRef {
	if lit == nil || len(path) == 0 {
		return nil
	}
	var args []midir.LocalID
	idx := path[0]
	elem, constElem := elemIndex(idx)
	switch {
	case idx == anyElem && lit.Op == midir.OpArray:
		args = lit.Args
	case constElem && lit.Op == midir.OpArray && elem < len(lit.Args):
		args = lit.Args[elem : elem+1]
	case idx >= 0 && lit.Op == midir.OpRecord && idx < len(lit.Args):
		args = lit.Args[idx : idx+1]
	case idx < 0 && idx > anyElem && !constElem && lit.Op == midir.OpVariant && -1-idx < len(lit.Args):
		args = lit.Args[-1-idx : -idx]
	default:
		return nil
	}
	var out []ownerRef
	for _, a := range args {
		part := c.holderOwners(a)
		if next := c.tempLiteral(a); len(path) > 1 && next != nil {
			if part = c.partHeld(next, path[1:]); part == nil {
				return nil
			}
		}
		out = append(out, part...)
	}
	return out
}

func (c *borrowCheck) literalOf(l midir.LocalID) *midir.Inst {
	d := c.soleDef(l)
	switch {
	case d == nil:
		return nil
	case literalOp(d.Op):
		return d
	case (d.Op == midir.OpCopy || d.Op == midir.OpMove) && len(d.Args) > 0:
		return c.tempLiteral(d.Args[0])
	}
	return c.arrayOfLiteral(d)
}

func literalOp(op midir.Opcode) bool {
	return op == midir.OpRecord || op == midir.OpVariant || op == midir.OpArray
}

func (c *borrowCheck) tempLiteral(l midir.LocalID) *midir.Inst {
	for range 8 {
		d := c.definition(l)
		if c.f.Locals[l].Ent != 0 || d == nil {
			return nil
		}
		switch {
		case literalOp(d.Op):
			return d
		case d.Op == midir.OpCopy || d.Op == midir.OpMove || d.Op == midir.OpAlias:
			l = d.Args[0]
		default:
			return c.arrayOfLiteral(d)
		}
	}
	return nil
}

// `Array.of(..)` keeps its literal's elements in order.
func (c *borrowCheck) arrayOfLiteral(d *midir.Inst) *midir.Inst {
	if d.Op != midir.OpCall || d.Ent == 0 || len(d.Args) != 1 {
		return nil
	}
	if e := c.p.R.Entity(d.Ent); e.Kind != sem.EntFn || e.Name != "of" || c.p.R.Fn(d.Ent).Owner != c.p.R.Types.ArrayEnt() {
		return nil
	}
	if a := c.definition(d.Args[0]); a != nil && a.Op == midir.OpArray && c.f.Locals[d.Args[0]].Ent == 0 {
		return a
	}
	return nil
}
