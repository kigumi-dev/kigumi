package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) verifyCellValues(f *midir.Func) *Error {
	if !p.hasCells(f) {
		return nil
	}
	fresh, cells := cellLocals(f, true), cellLocals(f, false)
	tt := p.R.Types
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			switch {
			case in.Op == midir.OpCellTake && !fresh[in.Args[0]]:
				return fail("cellops", midir.BlockID(bid), i, "celltake empties %%%s, a cell this frame did not make", f.Locals[in.Args[0]].Name)
			case in.Op == midir.OpNewCell && cells[in.Args[0]] && tt.Kind(f.Locals[in.Dst].Type) != sem.KRef:
				return fail("cellops", midir.BlockID(bid), i, "newcell wraps %%%s, which is already a cell", f.Locals[in.Args[0]].Name)
			}
			if in.Op.Yields() && f.Locals[in.Dst].Cell && !p.cellDef(f, in) {
				return fail("cellops", midir.BlockID(bid), i, "%s sets the cell %%%s to a value that is no cell", in.Op, f.Locals[in.Dst].Name)
			}
			if k := p.mutReceiver(in); k >= 0 && k < len(in.Args) && !p.canBeCell(f, in.Args[k], map[midir.LocalID]bool{}) {
				return fail("cellops", midir.BlockID(bid), i, "%s hands %%%s, which is no cell, to a mut receiver", in.Op, f.Locals[in.Args[k]].Name)
			}
			for k, a := range in.Args {
				if readsValue(in.Op, k) && cells[a] && tt.Kind(f.Locals[a].Type) != sem.KRef {
					return fail("cellops", midir.BlockID(bid), i, "%s reads the cell %%%s as a plain value", in.Op, f.Locals[a].Name)
				}
			}
		}
	}
	return nil
}

func (p *verifier) cellDef(f *midir.Func, in midir.Inst) bool {
	switch in.Op {
	case midir.OpNewCell:
		return true
	case midir.OpAlias, midir.OpShare, midir.OpCopy, midir.OpMove:
		return p.canBeCell(f, in.Args[0], map[midir.LocalID]bool{})
	}
	return false
}

func (p *verifier) mutReceiver(in midir.Inst) int {
	if (in.Op != midir.OpCall && in.Op != midir.OpCallValue) || in.Ent == 0 || int(in.Ent) >= len(p.R.Entities) || p.R.Entity(in.Ent).Kind != sem.EntFn || p.R.Fn(in.Ent).Recv != sem.RecvMut {
		return -1
	}
	if in.Op == midir.OpCallValue {
		return 1
	}
	return 0
}

func (p *verifier) hasCells(f *midir.Func) bool {
	for _, l := range f.Locals {
		if l.Cell {
			return true
		}
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpNewCell || in.Op == midir.OpCellTake || p.mutReceiver(in) >= 0 {
				return true
			}
		}
	}
	return false
}

func readsValue(op midir.Opcode, k int) bool {
	switch op {
	case midir.OpField, midir.OpFieldMove, midir.OpTag, midir.OpPayload, midir.OpIsVariant, midir.OpIsType, midir.OpUnbox, midir.OpBox, midir.OpUnary, midir.OpBoxDirect, midir.OpUnboxDirect:
		return k == 0
	case midir.OpIndex, midir.OpBinary, midir.OpSetField, midir.OpSetIndex, midir.OpRecord, midir.OpVariant, midir.OpArray:
		return true
	case midir.OpCellSet:
		return k == 1
	}
	return false
}

func cellLocals(f *midir.Func, fresh bool) []bool {
	n := len(f.Locals)
	defs, cellDefs := make([]int, n), make([]int, n)
	is := make([]bool, n)
	if !fresh {
		for l := range f.Locals {
			is[l] = f.Locals[l].Cell
		}
	}
	for changed := true; changed; {
		changed = false
		clear(cellDefs)
		clear(defs)
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() {
					continue
				}
				defs[in.Dst]++
				switch in.Op {
				case midir.OpNewCell:
					cellDefs[in.Dst]++
				case midir.OpAlias, midir.OpShare, midir.OpCopy, midir.OpMove:
					if is[in.Args[0]] {
						cellDefs[in.Dst]++
					}
				}
			}
		}
		for l := range is {
			if !is[l] && defs[l] > 0 && defs[l] == cellDefs[l] && !isParam(f, midir.LocalID(l)) && (!fresh || f.Locals[l].Ent == 0) {
				is[l], changed = true, true
			}
		}
	}
	return is
}
