package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// hoistRefReleases moves the scope-end release of a named reference used in one block up to its
// last use there; releasing a borrow of a named local runs no destructor, so only the borrow's end moves.
func hoistRefReleases(p *Program, f *Func) {
	var defs map[LocalID][]Inst
	var flow *blockFlow
	for l := range f.Locals {
		loc := f.Locals[l]
		if loc.Ent == 0 || loc.Cell || loc.Borrowed || p.R.Types.Kind(loc.Type) != sem.KRef || p.R.IsCopy(p.R.Types.Node(loc.Type).Elem) {
			continue
		}
		if defs == nil {
			defs = resultDefs(f)
		}
		if namedReferent(p, f, defs, LocalID(l), map[LocalID]bool{}) && !hoistRefRelease(p, f, LocalID(l)) && callResult(defs, LocalID(l)) {
			if flow == nil || len(flow.preds) != len(f.Blocks) {
				flow = newBlockFlow(f)
			}
			hoistRefDrops(p, f, LocalID(l), flow)
		}
	}
}

func resultDefs(f *Func) map[LocalID][]Inst {
	defs := map[LocalID][]Inst{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if shapes[in.Op].result {
				defs[in.Dst] = append(defs[in.Dst], in)
			}
		}
	}
	return defs
}

func namedReferent(p *Program, f *Func, defs map[LocalID][]Inst, l LocalID, seen map[LocalID]bool) bool {
	if seen[l] {
		return true
	}
	seen[l] = true
	if len(defs[l]) == 0 {
		return slices.Contains(f.Params, l)
	}
	for _, d := range defs[l] {
		switch d.Op {
		case OpCopy, OpMove, OpAlias, OpShare, OpBorrow, OpField, OpIndex, OpPayload, OpCellGet, OpNewCell:
			if a := d.Args[0]; f.Locals[a].Ent == 0 && !namedReferent(p, f, defs, a, seen) {
				return false
			}
		case OpCall, OpCallValue:
			if p.R.Types.Kind(f.Locals[l].Type) != sem.KRef {
				return false
			}
			for _, a := range d.Args {
				if p.R.Types.Kind(f.Locals[a].Type) == sem.KRef && f.Locals[a].Ent == 0 && !namedReferent(p, f, defs, a, seen) {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// hoistRefRelease moves l's one release to its defining block when every use is there, false when it cannot.
func hoistRefRelease(p *Program, f *Func, l LocalID) bool {
	home, def, last := -1, -1, -1
	dropBlk, drop := -1, -1
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		for i, in := range blk.Insts {
			switch {
			case shapes[in.Op].result && in.Dst == l:
				if home != -1 {
					return false
				}
				home, def = bid, i
			case in.Op == OpDrop && len(in.Args) == 1 && in.Args[0] == l:
				if !in.EndScope || drop != -1 {
					return false
				}
				dropBlk, drop = bid, i
			case slices.Contains(in.Args, l):
				if bid != home || shapes[in.Op].result && !p.R.IsCopy(f.Locals[in.Dst].Type) {
					return false
				}
				last = i
			}
		}
		if slices.Contains(blk.Term.Args, l) {
			return false
		}
	}
	if drop == -1 || home == -1 {
		return true
	}
	at := max(def, last) + 1
	if dropBlk == home && (last > drop || at >= drop) {
		return true
	}
	f.insertInst(home, at, f.deleteInst(dropBlk, drop))
	return true
}
