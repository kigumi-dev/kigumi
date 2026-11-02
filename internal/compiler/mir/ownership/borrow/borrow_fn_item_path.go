package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// fnItemPath are the fn items at path in l: a field step takes one field or payload, a type argument step every part
// whose declared type names that parameter, so a part that cannot hold such a value is never followed.
func (c *borrowCheck) fnItemPath(l midir.LocalID, path []sem.PartStep, depth int) fnItems {
	if len(path) == 0 {
		return c.partItems(l, depth)
	}
	again := func(a midir.LocalID, depth int) fnItems { return c.fnItemPath(a, path, depth) }
	return c.builtItems(l, path, depth, again, func(d *midir.Inst, depth int) fnItems { return c.builtPath(d, path, depth) })
}

func (c *borrowCheck) builtPath(d *midir.Inst, path []sem.PartStep, depth int) fnItems {
	step, rest := path[0], path[1:]
	if d.Op == midir.OpArray {
		if step.Field || step.Index != 0 {
			return fnItems{}
		}
		out := knownItems()
		for _, a := range d.Args {
			out = out.join(c.fnItemPath(a, rest, depth))
		}
		return out
	}
	if step.Field {
		switch {
		case step.Index < len(d.Args):
			return c.fnItemPath(d.Args[step.Index], rest, depth)
		case d.Op == midir.OpVariant:
			return knownItems()
		}
		return fnItems{}
	}
	declared, p, ok := c.declaredParts(d, step.Index)
	if !ok {
		return fnItems{}
	}
	out := knownItems()
	for i, a := range d.Args {
		paths, exact := c.paramPaths(declared[i], p)
		if !exact {
			return fnItems{}
		}
		for _, q := range paths {
			out = out.join(c.fnItemPath(a, slices.Concat(q, rest), depth))
		}
	}
	return out
}

// declaredParts are the declared types of the parts record or variant literal d builds, and the type parameter its type
// takes argument i for.
func (c *borrowCheck) declaredParts(d *midir.Inst, i int) ([]sem.TypeID, sem.TypeID, bool) {
	r, tt := c.p.R, c.p.R.Types
	n := tt.Node(d.Type)
	if n.Kind != sem.KNamed || i >= len(r.TypeDecl(n.Ent).Params) {
		return nil, 0, false
	}
	var declared []sem.TypeID
	switch d.Op {
	case midir.OpRecord:
		for _, f := range r.TypeDecl(n.Ent).Fields {
			declared = append(declared, r.Entity(f).Type)
		}
	case midir.OpVariant:
		declared = r.Variant(d.Ent).Payload
	default:
		return nil, 0, false
	}
	if len(declared) != len(d.Args) {
		return nil, 0, false
	}
	return declared, tt.Param(r.TypeDecl(n.Ent).Params[i]), true
}

// paramPaths are the type argument paths at which declared type t names p; exact is false when t also names p where no
// path leads, under a fn, a reference or an interface, whose values a proof cannot follow.
func (c *borrowCheck) paramPaths(t, p sem.TypeID) (paths [][]sem.PartStep, exact bool) {
	tt := c.p.R.Types
	n := tt.Node(t)
	switch {
	case t == p:
		return [][]sem.PartStep{nil}, true
	case n.Kind != sem.KNamed:
		return nil, !tt.Contains(t, p)
	}
	for i, a := range n.Args {
		sub, ok := c.paramPaths(a, p)
		if !ok {
			return nil, false
		}
		for _, s := range sub {
			paths = append(paths, append([]sem.PartStep{{Index: i}}, s...))
		}
	}
	return paths, true
}
