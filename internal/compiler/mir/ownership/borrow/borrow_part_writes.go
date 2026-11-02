package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// partWrite is a write into a local at path (fields and payloads), anywhere below it when deep: a store of vals, or a
// lend of the place to argument arg of call, which is nil when the borrow reaches no call.
type partWrite struct {
	path []sem.PartStep
	deep bool
	vals []midir.LocalID
	lend bool
	call *midir.Inst
	arg  int
}

// placeAt is where a projection of a local points: deep when its exact part is not known below path.
type placeAt struct {
	path []sem.PartStep
	deep bool
}

type writeSet struct {
	writes []partWrite
	ok     bool
}

// writtenThrough reports whether something stores into l or into a part of it, or borrows such a place mutably.
func (c *borrowCheck) writtenThrough(l midir.LocalID) bool {
	writes, ok := c.partWrites(l)
	return !ok || len(writes) > 0
}

// partWrites are the writes into l or into a part of it reached through a field, element, payload, alias or shared
// borrow; false when one cannot be followed: a cell, or a mutable borrow put to a use other than one call.
func (c *borrowCheck) partWrites(l midir.LocalID) ([]partWrite, bool) {
	if s, ok := c.partWritesOf[l]; ok {
		return s.writes, s.ok
	}
	if c.partWritesOf == nil {
		c.partWritesOf = map[midir.LocalID]writeSet{}
	}
	writes, ok := c.collectPartWrites(c.projectionsOf(l))
	c.partWritesOf[l] = writeSet{writes, ok}
	return writes, ok
}

func (c *borrowCheck) collectPartWrites(places map[midir.LocalID]placeAt) ([]partWrite, bool) {
	var writes []partWrite
	for p := range places {
		if c.f.Locals[p].Cell {
			return nil, false
		}
	}
	for _, blk := range c.f.Blocks {
		for i := range blk.Insts {
			in := &blk.Insts[i]
			if len(in.Args) == 0 {
				continue
			}
			at, ok := places[in.Args[0]]
			switch {
			case !ok:
			case in.Op == midir.OpSetField && at.deep:
				writes = append(writes, partWrite{path: at.path, deep: true, vals: in.Args[1:]})
			case in.Op == midir.OpSetField:
				writes = append(writes, partWrite{path: slices.Concat(at.path, []sem.PartStep{{Index: in.Index, Field: true}}), vals: in.Args[1:]})
			case in.Op == midir.OpSetIndex:
				writes = append(writes, partWrite{path: at.path, deep: true, vals: in.Args[1:]})
			case in.Op == midir.OpCellSet:
				return nil, false
			case in.Op == midir.OpBorrow && in.Index == 1:
				call, arg, ok := c.lentCall(in.Dst)
				if !ok {
					return nil, false
				}
				writes = append(writes, partWrite{path: at.path, deep: at.deep, lend: true, call: call, arg: arg})
			}
		}
	}
	return writes, true
}

// projectionsOf maps l and every local projecting a part of it to where it points; a local reached two ways points
// anywhere below what both ways share.
func (c *borrowCheck) projectionsOf(l midir.LocalID) map[midir.LocalID]placeAt {
	places := map[midir.LocalID]placeAt{l: {}}
	for changed := true; changed; {
		changed = false
		for _, blk := range c.f.Blocks {
			for _, in := range blk.Insts {
				if len(in.Args) == 0 || !in.Op.Yields() || in.Op == midir.OpBorrow && in.Index == 1 {
					continue
				}
				at, ok := places[in.Args[0]]
				if !ok {
					continue
				}
				next, ok := projectStep(in, at)
				if !ok {
					continue
				}
				if old, seen := places[in.Dst]; seen {
					next = old.meet(next)
					if next.deep == old.deep && len(next.path) == len(old.path) {
						continue
					}
				}
				places[in.Dst], changed = next, true
			}
		}
	}
	return places
}

func projectStep(in midir.Inst, at placeAt) (placeAt, bool) {
	switch in.Op {
	case midir.OpField, midir.OpFieldMove, midir.OpPayload:
		if at.deep {
			return at, true
		}
		return placeAt{path: slices.Concat(at.path, []sem.PartStep{{Index: in.Index, Field: true}})}, true
	case midir.OpIndex:
		return placeAt{path: at.path, deep: true}, true
	case midir.OpAlias, midir.OpBorrow:
		return at, true
	}
	return placeAt{}, false
}

func (a placeAt) meet(b placeAt) placeAt {
	n := 0
	for n < len(a.path) && n < len(b.path) && a.path[n] == b.path[n] {
		n++
	}
	if n == len(a.path) && n == len(b.path) {
		return placeAt{path: a.path, deep: a.deep || b.deep}
	}
	return placeAt{path: a.path[:n:n], deep: true}
}

// lentCall is the one call mutable borrow b is handed to, directly or in the cell a `mut self` receiver takes, and the
// argument it fills; false for any other use.
func (c *borrowCheck) lentCall(b midir.LocalID) (*midir.Inst, int, bool) {
	held := map[midir.LocalID]bool{b: true}
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpNewCell && in.Args[0] == b {
				held[in.Dst] = true
			}
		}
	}
	var call *midir.Inst
	arg := 0
	for _, blk := range c.f.Blocks {
		if slices.ContainsFunc(blk.Term.Args, func(a midir.LocalID) bool { return held[a] }) {
			return nil, 0, false
		}
		for i := range blk.Insts {
			in := &blk.Insts[i]
			for k, a := range in.Args {
				switch {
				case !held[a]:
				case in.Op == midir.OpDrop || in.Op == midir.OpNewCell && a == b && held[in.Dst]:
				case in.Op == midir.OpCall && call == nil:
					call, arg = in, k
				default:
					return nil, 0, false
				}
			}
		}
	}
	return call, arg, true
}
