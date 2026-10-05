package mir

// verifyCellCaptures checks that a closure capturing a local as a cell
// actually receives one: build_scope.go's param() and bindPattern always
// back a Cell local with an OpNewCell before it is used, so a mismatch
// would hand the closure a plain value where it expects a cell, crashing
// OpCellGet/OpCellSet on the other side.
func (p *Program) verifyCellCaptures(f *Func) *Error {
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			if in.Op != OpClosure {
				continue
			}
			callee := p.ByEnt[in.Ent]
			if callee == nil {
				continue
			}
			for k, arg := range in.Args {
				if k >= len(callee.Params) || !callee.Locals[callee.Params[k]].Cell {
					continue
				}
				if !definedByNewCell(f, arg) {
					return fail("cellcapture", bid, i, "captures %%%s as a cell, but %%%s was never wrapped in one (OpNewCell)", f.Locals[arg].Name, f.Locals[arg].Name)
				}
			}
		}
	}
	return nil
}

// definedByNewCell traces l back through plan_rc.go's alias/share/copy chain to
// the OpNewCell it must come from; a Cell-flagged parameter ends the trace.
func definedByNewCell(f *Func, l LocalID) bool {
	seen := map[LocalID]bool{}
	for {
		if seen[l] {
			return false
		}
		seen[l] = true
		if isParam(f, l) && f.Locals[l].Cell {
			return true
		}
		found := false
		// Only Yields() can define l: a void op's Dst is LocalID(0), which
		// would otherwise match when l is 0.
	search:
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || in.Dst != l {
					continue
				}
				switch in.Op {
				case OpNewCell:
					return true
				case OpAlias, OpShare, OpCopy, OpMove, OpBorrow:
					l, found = in.Args[0], true
				default:
					return false
				}
				break search
			}
		}
		if !found {
			return false
		}
	}
}

func isParam(f *Func, l LocalID) bool {
	for _, prm := range f.Params {
		if prm == l {
			return true
		}
	}
	return false
}
