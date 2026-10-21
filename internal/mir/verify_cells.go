package mir

import "kigumi/internal/sem"

// verifyCellCaptures checks that a closure capturing a local as a cell
// actually receives one: build_scope.go's param() and bindPattern always
// back a Cell local with an OpNewCell before it is used, so a mismatch
// would hand the closure a plain value where it expects a cell, crashing
// OpCellGet/OpCellSet on the other side.
func (p *Program) verifyCellCaptures(f *Func) *Error {
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			if in.Op != OpClosure || in.Str == "applied" {
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

func (p *Program) verifyCellOps(f *Func) *Error {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if (in.Op == OpCellGet || in.Op == OpCellTake || in.Op == OpCellSet) && !p.canBeCell(f, in.Args[0], map[LocalID]bool{}) {
				return fail("cellops", BlockID(bid), i, "%s through %%%s, which is no cell", in.Op, f.Locals[in.Args[0]].Name)
			}
		}
	}
	return nil
}

func yielders(f *Func) map[LocalID][]Inst {
	if f.yielders == nil {
		f.yielders = map[LocalID][]Inst{}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op.Yields() {
					f.yielders[in.Dst] = append(f.yielders[in.Dst], in)
				}
			}
		}
	}
	return f.yielders
}

func (p *Program) canBeCell(f *Func, l LocalID, seen map[LocalID]bool) bool {
	if seen[l] || f.Locals[l].Cell || (isParam(f, l) && p.refCellParam(f, l)) {
		return true
	}
	seen[l] = true
	defined := false
	for _, in := range yielders(f)[l] {
		defined = true
		switch in.Op {
		case OpConst, OpUnit, OpBinary, OpUnary, OpInterp, OpShell, OpTag, OpIsVariant, OpIsType, OpRecord, OpVariant, OpArray, OpClosure, OpFnItem, OpBind, OpCFnPtr, OpBox, OpLendBox, OpBoxDirect, OpUnboxDirect,
			OpCall, OpCallValue, OpCallC, OpBuiltin, OpAwait, OpFutureNew, OpCellGet, OpCellTake, OpAsm,
			OpField, OpFieldMove, OpPayload, OpIndex, OpUnbox, OpGlobalGet:
		case OpAlias, OpShare, OpCopy, OpMove, OpBorrow:
			if p.canBeCell(f, in.Args[0], seen) {
				return true
			}
		default:
			return true
		}
	}
	if isParam(f, l) {
		return f.Locals[l].Borrowed || f.Locals[l].Cell
	}
	return !defined
}

func (p *Program) cellContent(f *Func, l LocalID, seen map[LocalID]bool) []sem.TypeID {
	if seen[l] {
		return nil
	}
	seen[l] = true
	t := f.Locals[l].Type
	tt := p.R.Types
	if isParam(f, l) {
		switch {
		case f.Locals[l].Cell:
			return []sem.TypeID{t}
		case tt.Kind(t) == sem.KRef:
			return []sem.TypeID{tt.Node(t).Elem}
		case f.Locals[l].Borrowed:
			return []sem.TypeID{t}
		}
		return nil
	}
	var found []sem.TypeID
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() || in.Dst != l {
				continue
			}
			var c []sem.TypeID
			switch in.Op {
			case OpNewCell:
				c = []sem.TypeID{t}
				if tt.Kind(t) == sem.KRef {
					c = append(c, tt.Node(t).Elem)
				}
			case OpAlias, OpShare, OpCopy, OpMove, OpBorrow:
				c = p.cellContent(f, in.Args[0], seen)
			}
			if len(c) == 0 {
				return nil
			}
			found = append(found, c...)
		}
	}
	return found
}

func (p *Program) cellFits(f *Func, l LocalID, v sem.TypeID) (bool, sem.TypeID) {
	content := p.cellContent(f, l, map[LocalID]bool{})
	for _, c := range content {
		if p.fits(c, v) {
			return true, 0
		}
	}
	if len(content) == 0 {
		return true, 0
	}
	return false, content[0]
}

func (p *Program) refCellParam(f *Func, l LocalID) bool {
	t := f.Locals[l].Type
	if _, scalar := p.R.ScalarRefElem(t); scalar {
		return true
	}
	_, mutRef := p.R.MutRefElem(t)
	return mutRef && f.Locals[l].Ent != 0 && p.R.RefCellParam(f.Locals[l].Ent)
}
