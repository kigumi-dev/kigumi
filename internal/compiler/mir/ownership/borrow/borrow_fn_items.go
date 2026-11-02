package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// fnItems is what a callable value can be when known: its fn items, none only when the value's structure shows it holds
// no callable. A value whose origin is lost is unknown.
type fnItems struct {
	ents  []sem.EntityID
	known bool
}

func knownItems(ents ...sem.EntityID) fnItems { return fnItems{ents: ents, known: true} }

func (s fnItems) join(o fnItems) fnItems {
	if !s.known || !o.known {
		return fnItems{}
	}
	return knownItems(append(s.ents[:len(s.ents):len(s.ents)], o.ents...)...)
}

// fnItemOrigins are the fn items l holds: through copies, joins, a field, element or payload of a container built here,
// and a call result its callee's signature relays from an argument.
func (c *borrowCheck) fnItemOrigins(l midir.LocalID, depth int) fnItems {
	defs := c.defsOf(l)
	if depth == 0 || len(defs) == 0 || c.f.Locals[l].Cell {
		return fnItems{}
	}
	out := knownItems()
	for _, d := range defs {
		var ents fnItems
		switch {
		case d.Op == midir.OpFnItem:
			ents = knownItems(d.Ent)
		case len(d.Args) == 0:
		case d.Op == midir.OpCopy || d.Op == midir.OpMove || d.Op == midir.OpAlias:
			ents = c.fnItemOrigins(d.Args[0], depth-1)
		case d.Op == midir.OpCall:
			ents = c.relayedFnItems(d, depth-1, c.fnItemOrigins)
		case d.Op == midir.OpField || d.Op == midir.OpFieldMove || d.Op == midir.OpPayload:
			ents = c.fnItemPath(d.Args[0], []sem.PartStep{{Index: d.Index, Field: true}}, depth-1)
		case d.Op == midir.OpIndex:
			ents = c.fnItemsWithin(d.Args[0], depth-1)
		}
		if out = out.join(ents); !out.known {
			return out
		}
	}
	return out
}

// fnItemsWithin are the fn items anywhere in the container l, l being built here and never written through.
func (c *borrowCheck) fnItemsWithin(l midir.LocalID, depth int) fnItems {
	return c.builtItems(l, nil, depth, func(a midir.LocalID, depth int) fnItems { return c.fnItemsWithin(a, depth) },
		func(d *midir.Inst, depth int) fnItems {
			out := knownItems()
			for _, a := range d.Args {
				out = out.join(c.partItems(a, depth))
			}
			return out
		})
}

// partItems are the fn items a part a holds, whether it is a callable or a container of them.
func (c *borrowCheck) partItems(a midir.LocalID, depth int) fnItems {
	if c.holdsNoCallable(c.f.Locals[a].Type, map[sem.TypeID]bool{}) {
		return knownItems()
	}
	ents := c.fnItemOrigins(a, depth)
	if !ents.known && c.slotFnType(c.f.Locals[a].Type) == 0 {
		ents = c.fnItemsWithin(a, depth)
	}
	return ents
}

// builtItems are the fn items at path of the container l: where it is built, through copies and generic relays (built
// reads a record, variant or array literal), and what the writes into it can leave there.
func (c *borrowCheck) builtItems(l midir.LocalID, path []sem.PartStep, depth int, again func(midir.LocalID, int) fnItems, built func(*midir.Inst, int) fnItems) fnItems {
	if c.holdsNoCallable(c.f.Locals[l].Type, map[sem.TypeID]bool{}) {
		return knownItems()
	}
	defs := c.defsOf(l)
	if depth == 0 || len(defs) == 0 {
		return fnItems{}
	}
	writes, ok := c.partWrites(l)
	if !ok {
		return fnItems{}
	}
	out := c.writesItems(c.f.Locals[l].Type, writes, path, depth-1)
	if !out.known {
		return out
	}
	for _, d := range defs {
		var ents fnItems
		switch {
		case d.Op == midir.OpRecord || d.Op == midir.OpVariant || d.Op == midir.OpArray:
			ents = built(d, depth-1)
		case len(d.Args) == 0:
		case d.Op == midir.OpCopy || d.Op == midir.OpMove || d.Op == midir.OpAlias:
			ents = again(d.Args[0], depth-1)
		case d.Op == midir.OpCall && c.p.R.Entity(d.Ent).Name == "of" && c.p.R.Fn(d.Ent).Owner == c.p.R.Types.ArrayEnt():
			ents = again(d.Args[0], depth-1)
		case d.Op == midir.OpCall:
			ents = c.relayedFnItems(d, depth-1, again)
		}
		if out = out.join(ents); !out.known {
			return out
		}
	}
	return out
}
