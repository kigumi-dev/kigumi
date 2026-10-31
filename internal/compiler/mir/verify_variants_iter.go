package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func iterAt(name string) bool {
	s, ok := opdef.BuiltinFor(name)
	return ok && s.Iter && len(s.Args) == 2
}

func iterLengths(f *midir.Func) map[midir.LocalID]midir.LocalID {
	defs := map[midir.LocalID]int{}
	lens := map[midir.LocalID]midir.LocalID{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			defs[in.Dst]++
			if in.Op == midir.OpBuiltin && in.Str == "iter.len" && len(in.Args) == 1 {
				lens[in.Dst] = in.Args[0]
			}
		}
	}
	for l := range lens {
		if defs[l] != 1 {
			delete(lens, l)
		}
	}
	return lens
}

func lessPlace(a, b midir.LocalID) vplace { return vplace{root: a, rel: b, tag: tagBelow} }

func (vf *variantFlow) inBounds(s *variantFacts, in midir.Inst) bool {
	for pl := range s.known {
		if pl.tag == tagBelow && pl.root == in.Args[1] && vf.lenOf[pl.rel] == in.Args[0] {
			return true
		}
	}
	return false
}

var emptyCell = variantSet{list: []sem.EntityID{1}, all: 2}

func cellPlace(pl vplace) vplace { return vplace{root: pl.root, path: pl.path, tag: tagEmptied} }

func (s *variantFacts) emptied(in midir.Inst) (midir.LocalID, bool) {
	if len(s.known) == 0 {
		return 0, false
	}
	switch in.Op {
	case midir.OpCellGet, midir.OpCellTake, midir.OpCall, midir.OpCallValue:
		for _, a := range in.Args {
			if _, ok := s.known[cellPlace(s.where(a))]; ok {
				return a, true
			}
		}
	}
	return 0, false
}

func carriedBools(f *midir.Func, cond []bool) []bool {
	carried := make([]bool, len(f.Locals))
	defs := make([]int, len(f.Locals))
	where := make([]int, len(f.Locals))
	for bid, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && cond[in.Dst] {
				defs[in.Dst]++
				where[in.Dst] = bid
			}
		}
	}
	for l, n := range defs {
		carried[l] = n > 1
	}
	for bid, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				if cond[a] && (defs[a] == 0 || where[a] != bid) {
					carried[a] = true
				}
			}
		}
		for _, a := range blk.Term.Args {
			if cond[a] && (defs[a] == 0 || where[a] != bid) {
				carried[a] = true
			}
		}
	}
	return carried
}

// cellReadAfterTake finds a cell read after, or in another block than, a
// celltake of it; the usual plumbing takes the cell back last.
func cellReadAfterTake(f *midir.Func) bool {
	for bid, blk := range f.Blocks {
		for k, in := range blk.Insts {
			if in.Op != midir.OpCellTake {
				continue
			}
			c := in.Args[0]
			for j, later := range blk.Insts {
				if j > k && readsCell(later, c) {
					return true
				}
				if j > k && later.Op == midir.OpCellSet && later.Args[0] == c {
					break
				}
			}
			for ob, other := range f.Blocks {
				if ob == bid {
					continue
				}
				for _, later := range other.Insts {
					if readsCell(later, c) {
						return true
					}
				}
			}
		}
	}
	return false
}

func readsCell(in midir.Inst, c midir.LocalID) bool {
	switch in.Op {
	case midir.OpCellGet, midir.OpCellTake, midir.OpCall, midir.OpCallValue:
		return slices.Contains(in.Args, c)
	}
	return false
}
