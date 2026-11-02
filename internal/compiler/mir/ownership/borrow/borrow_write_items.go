package borrow

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// writesItems are the fn items the writes into a local of type t can leave at path q of it.
func (c *borrowCheck) writesItems(t sem.TypeID, writes []partWrite, q []sem.PartStep, depth int) fnItems {
	out := knownItems()
	for _, w := range writes {
		if out = out.join(c.writeItems(t, w, w.path, q, depth)); !out.known {
			return out
		}
	}
	return out
}

// writeItems follows the rest of w's path from a place of type t and path q from the same place through the declared
// field types, as builtPath does for a literal: a field q cannot reach gets nothing.
func (c *borrowCheck) writeItems(t sem.TypeID, w partWrite, rest, q []sem.PartStep, depth int) fnItems {
	if len(rest) == 0 {
		return c.writtenItems(w, t, q, depth)
	}
	step, sub := rest[0], c.fieldType(t, rest[0].Index)
	switch {
	case len(q) == 0:
		return c.writeItems(sub, w, rest[1:], nil, depth)
	case q[0].Field && q[0].Index != step.Index:
		return knownItems()
	case q[0].Field:
		return c.writeItems(sub, w, rest[1:], q[1:], depth)
	}
	declared, p, ok := c.recordField(t, step.Index, q[0].Index)
	if !ok {
		return fnItems{}
	}
	paths, exact := c.paramPaths(declared, p)
	if !exact {
		return fnItems{}
	}
	out := knownItems()
	for _, path := range paths {
		if out = out.join(c.writeItems(sub, w, rest[1:], slices.Concat(path, q[1:]), depth)); !out.known {
			return out
		}
	}
	return out
}

// writtenItems are the fn items w leaves at path q of the place it writes. A deep store may land anywhere below, so
// everything its values hold counts; a deep lend, or one whose result can reach the place, is unknown.
func (c *borrowCheck) writtenItems(w partWrite, t sem.TypeID, q []sem.PartStep, depth int) fnItems {
	switch {
	case w.lend && w.call == nil:
		return knownItems()
	case w.lend && (w.deep || c.lendReturns(w.call, w.arg)):
		return fnItems{}
	case w.lend:
		return c.lentItems(w.call, w.arg, t, q, depth)
	case w.deep:
		out := knownItems()
		for _, v := range w.vals {
			out = out.join(c.partItems(v, depth))
		}
		return out
	}
	return c.fnItemPath(w.vals[0], q, depth)
}

// recordField is the declared type of field i of record type t and the type parameter t's argument k stands for.
func (c *borrowCheck) recordField(t sem.TypeID, i, k int) (sem.TypeID, sem.TypeID, bool) {
	r, tt := c.p.R, c.p.R.Types
	n := tt.Node(t)
	if n.Kind != sem.KNamed {
		return 0, 0, false
	}
	decl := r.TypeDecl(n.Ent)
	if i >= len(decl.Fields) || k >= len(decl.Params) {
		return 0, 0, false
	}
	return r.Entity(decl.Fields[i]).Type, tt.Param(decl.Params[k]), true
}

// fieldType is the type of field i of a value of record type t, 0 when t is not a record with that field.
func (c *borrowCheck) fieldType(t sem.TypeID, i int) sem.TypeID {
	r, tt := c.p.R, c.p.R.Types
	n := tt.Node(t)
	if t == 0 || n.Kind != sem.KNamed || i >= len(r.TypeDecl(n.Ent).Fields) {
		return 0
	}
	decl := r.TypeDecl(n.Ent)
	subst := map[sem.EntityID]sem.TypeID{}
	for k, p := range decl.Params {
		if k < len(n.Args) {
			subst[p] = n.Args[k]
		}
	}
	return tt.Subst(r.Entity(decl.Fields[i]).Type, subst)
}
