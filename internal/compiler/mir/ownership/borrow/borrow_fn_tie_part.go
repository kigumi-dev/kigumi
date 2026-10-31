package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

type wbKind uint8

const (
	wbWhole wbKind = iota
	wbField
	wbIndex
)

// writeBack is a `&mut` cell going back into a holder after a call: the whole holder, one field, or one element.
type writeBack struct {
	cell  midir.LocalID
	kind  wbKind
	field int
	index midir.LocalID
}

func (c *borrowCheck) writeBackOf(in midir.Inst, holder midir.LocalID) (writeBack, bool) {
	var wb writeBack
	var v midir.LocalID
	switch {
	case in.Op.Yields() && in.Dst == holder && (in.Op == midir.OpCopy || in.Op == midir.OpMove) && len(in.Args) > 0:
		v = in.Args[0]
	case in.Op == midir.OpSetField && len(in.Args) == 2 && in.Args[0] == holder:
		wb.kind, wb.field, v = wbField, in.Index, in.Args[1]
	case in.Op == midir.OpSetIndex && len(in.Args) == 3 && in.Args[0] == holder:
		wb.kind, wb.index, v = wbIndex, in.Args[1], in.Args[2]
	default:
		return wb, false
	}
	d := c.definition(v)
	for d != nil && d.Op == midir.OpAlias && len(d.Args) > 0 {
		d = c.definition(d.Args[0])
	}
	if d == nil || d.Op != midir.OpCellTake || len(d.Args) == 0 {
		return wb, false
	}
	wb.cell = d.Args[0]
	return wb, true
}

// partReplaced: insts[i] writes back a field or element of holder that the call stored on every path, and every tie
// of owner into holder came in through a write-back of that same part.
func (c *borrowCheck) partReplaced(bid midir.BlockID, i int, holder, owner midir.LocalID, sites []carrySite) bool {
	insts := c.f.Blocks[bid].Insts
	wb, ok := c.writeBackOf(insts[i], holder)
	if !ok || wb.kind == wbWhole || !c.partStable(holder) || !c.callStores(insts[:i], wb.cell) {
		return false
	}
	found := false
	for _, s := range sites {
		if s.owner != owner {
			continue
		}
		at, ok := c.writeBackOf(c.f.Blocks[s.block].Insts[s.idx], holder)
		if !ok || !c.samePart(wb, at) {
			return false
		}
		found = true
	}
	return found
}

func (c *borrowCheck) samePart(a, b writeBack) bool {
	switch {
	case a.kind != b.kind:
		return false
	case a.kind == wbField:
		return a.field == b.field
	case a.kind == wbIndex:
		// Only a constant names the same element each time; a loop gives one index temporary many values.
		x, y := c.definition(a.index), c.definition(b.index)
		return x != nil && y != nil && x.Op == midir.OpConst && y.Op == midir.OpConst && x.Lit.Int != nil && y.Lit.Int != nil && x.Lit.Int.Cmp(y.Lit.Int) == 0
	}
	return true
}

// partStable reports whether every `&mut` borrow of holder's places is of a field or element and goes into a cell
// written back there, so nothing but those write-backs and plain stores, which mark their own ties, moves a tie inside it.
func (c *borrowCheck) partStable(holder midir.LocalID) bool {
	if s, ok := c.ties.stable[holder]; ok {
		return s
	}
	back := map[midir.LocalID]bool{}
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			wb, ok := c.writeBackOf(in, holder)
			for l := wb.cell; ok && !back[l]; {
				back[l] = true
				d := c.definition(l)
				if d == nil || len(d.Args) == 0 || d.Op != midir.OpNewCell && d.Op != midir.OpShare && d.Op != midir.OpCopy && d.Op != midir.OpAlias {
					break
				}
				l = d.Args[0]
			}
		}
	}
	stable := !c.f.Locals[holder].Cell
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if !stable || in.Op != midir.OpBorrow || in.Index != 1 {
				continue
			}
			if root, path, exact, ok := c.projectionPlace(in.Args[0]); ok && root == holder && (exact && len(path) == 0 || !back[in.Dst]) {
				stable = false
			}
		}
	}
	if c.ties.stable == nil {
		c.ties.stable = map[midir.LocalID]bool{}
	}
	c.ties.stable[holder] = stable
	return stable
}
