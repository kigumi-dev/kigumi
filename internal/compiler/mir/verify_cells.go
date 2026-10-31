package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// verifyCellCaptures checks that a closure capturing a local as a cell
// actually receives one: build_scope.go's param() and bindPattern always
// back a Cell local with an OpNewCell before it is used, so a mismatch
// would hand the closure a plain value where it expects a cell, crashing
// OpCellGet/OpCellSet on the other side.
func (p *verifier) verifyCellCaptures(f *midir.Func) *Error {
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
		for i, in := range blk.Insts {
			if in.Op != midir.OpClosure || in.Str == "applied" {
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
func definedByNewCell(f *midir.Func, l midir.LocalID) bool {
	seen := map[midir.LocalID]bool{}
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
				case midir.OpNewCell:
					return true
				case midir.OpAlias, midir.OpShare, midir.OpCopy, midir.OpMove, midir.OpBorrow:
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

func isParam(f *midir.Func, l midir.LocalID) bool {
	for _, prm := range f.Params {
		if prm == l {
			return true
		}
	}
	return false
}

func (p *verifier) verifyCellOps(f *midir.Func) *Error {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if (in.Op == midir.OpCellGet || in.Op == midir.OpCellTake || in.Op == midir.OpCellSet) && !p.canBeCell(f, in.Args[0], map[midir.LocalID]bool{}) {
				return fail("cellops", midir.BlockID(bid), i, "%s through %%%s, which is no cell", in.Op, f.Locals[in.Args[0]].Name)
			}
		}
	}
	return nil
}

func (p *verifier) yielders(f *midir.Func) map[midir.LocalID][]midir.Inst {
	body := p.body(f)
	if body.yielders == nil {
		body.yielders = map[midir.LocalID][]midir.Inst{}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op.Yields() {
					body.yielders[in.Dst] = append(body.yielders[in.Dst], in)
				}
			}
		}
	}
	return body.yielders
}

func (p *verifier) canBeCell(f *midir.Func, l midir.LocalID, seen map[midir.LocalID]bool) bool {
	if seen[l] || f.Locals[l].Cell || (isParam(f, l) && p.refCellParam(f, l)) {
		return true
	}
	seen[l] = true
	defined := false
	for _, in := range p.yielders(f)[l] {
		defined = true
		switch in.Op {
		case midir.OpConst, midir.OpUnit, midir.OpBinary, midir.OpUnary, midir.OpInterp, midir.OpShell, midir.OpTag, midir.OpIsVariant, midir.OpIsType, midir.OpRecord, midir.OpVariant, midir.OpArray, midir.OpClosure, midir.OpFnItem, midir.OpBind, midir.OpCFnPtr, midir.OpBox, midir.OpLendBox, midir.OpBoxDirect, midir.OpUnboxDirect,
			midir.OpCall, midir.OpCallValue, midir.OpCallC, midir.OpBuiltin, midir.OpAwait, midir.OpFutureNew, midir.OpCellGet, midir.OpCellTake, midir.OpAsm,
			midir.OpField, midir.OpFieldMove, midir.OpPayload, midir.OpIndex, midir.OpUnbox, midir.OpGlobalGet:
		case midir.OpAlias, midir.OpShare, midir.OpCopy, midir.OpMove, midir.OpBorrow:
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

func (p *verifier) cellContent(f *midir.Func, defs cellDefs, l midir.LocalID, seen map[midir.LocalID]bool) []sem.TypeID {
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
	for _, in := range defs[l] {
		var c []sem.TypeID
		switch in.Op {
		case midir.OpNewCell:
			c = []sem.TypeID{t}
			if tt.Kind(t) == sem.KRef {
				c = append(c, tt.Node(t).Elem)
			}
		case midir.OpAlias, midir.OpShare, midir.OpCopy, midir.OpMove, midir.OpBorrow:
			c = p.cellContent(f, defs, in.Args[0], seen)
		}
		if len(c) == 0 {
			return nil
		}
		found = append(found, c...)
	}
	return found
}

func (p *verifier) cellFits(f *midir.Func, defs cellDefs, l midir.LocalID, v sem.TypeID) (bool, sem.TypeID) {
	content := p.cellContent(f, defs, l, map[midir.LocalID]bool{})
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

func (p *verifier) refCellParam(f *midir.Func, l midir.LocalID) bool {
	t := f.Locals[l].Type
	if _, scalar := p.R.ScalarRefElem(t); scalar {
		return true
	}
	_, mutRef := p.R.MutRefElem(t)
	return mutRef && f.Locals[l].Ent != 0 && p.R.RefCellParam(f.Locals[l].Ent)
}
