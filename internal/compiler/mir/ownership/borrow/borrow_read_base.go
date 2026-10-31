package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// readRoot is where a chain of field, payload and index reads starts: a reference, or with elems a named holder.
type readRoot struct {
	base  midir.LocalID
	path  ownquery.Path
	exact bool
	ref   bool
}

// refBase walks l's field, payload and index reads down to the reference they read through, with l's path below it.
func (c *borrowCheck) refBase(l midir.LocalID) (midir.LocalID, ownquery.Path, bool, bool) {
	r, ok := c.readBase(l, false)
	return r.base, r.path, r.exact, ok && r.ref && !slices.Contains(c.f.Params, r.base)
}

func (c *borrowCheck) readHopOf(d *midir.Inst) (midir.LocalID, readHop, bool) {
	r, ok := c.readBase(d.Args[0], true)
	step, has := ownquery.PathStep(d, c.definition)
	if !ok || !r.ref || !has {
		return r.base, readHop{}, ok && r.ref
	}
	return r.base, readHop{path: r.path.With(step), exact: r.exact}, true
}

// readBase is refBase that, with elems, keeps an index step as an element step and also stops at a named holder.
func (c *borrowCheck) readBase(l midir.LocalID, elems bool) (readRoot, bool) {
	r := readRoot{exact: true}
	for seen := map[midir.LocalID]bool{}; !seen[l]; {
		seen[l] = true
		if c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef {
			r.base, r.ref = l, true
			return r, true
		}
		if c.f.Locals[l].Ent != 0 {
			r.base = l
			return r, elems
		}
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 {
			return readRoot{}, false
		}
		switch in.Op {
		case midir.OpCopy:
		case midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpUnbox:
			def := c.definition
			if !elems {
				def = nil
			}
			r.path, r.exact = r.path.Under(in, r.exact, def)
		default:
			return readRoot{}, false
		}
		l = in.Args[0]
	}
	return readRoot{}, false
}
