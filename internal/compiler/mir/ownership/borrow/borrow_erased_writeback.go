package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

// lentCellPlace is the path of owner's place that write-back cell c lends.
func (e *erasedReaders) lentCellPlace(c, owner midir.LocalID) (ownquery.Path, bool) {
	d := e.defs[c]
	if d == nil || d.Op != midir.OpNewCell || len(d.Args) == 0 {
		return nil, false
	}
	b := d.Args[0]
	for d = e.defs[b]; d != nil && d.Op == midir.OpShare && len(d.Args) > 0; d = e.defs[b] {
		b = d.Args[0]
	}
	if d == nil || d.Op != midir.OpBorrow || d.Index != 1 {
		return nil, false
	}
	u, ok := cellAccess(e.f, e.defs, e.win.chained, owner, *d)
	return u.path, ok
}

// callWrites are the parts of owner's place a call may write through the write-back cells it takes: what the callee's
// body writes through that cell parameter, or the whole lent place when no lowered body says.
func (e *erasedReaders) callWrites(in midir.Inst, owner midir.LocalID) []ownquery.Path {
	if in.Op != midir.OpCall && in.Op != midir.OpCallValue {
		return nil
	}
	var out []ownquery.Path
	for k := range in.Args {
		out = append(out, e.argWrites(in, k, owner)...)
	}
	return out
}

func (e *erasedReaders) argWrites(in midir.Inst, k int, owner midir.LocalID) []ownquery.Path {
	lent, ok := e.lentCellPlace(in.Args[k], owner)
	if !ok {
		return nil
	}
	g := e.callee(in)
	if g == nil || !g.Locals[g.Params[k]].Cell {
		return []ownquery.Path{lent}
	}
	var out []ownquery.Path
	for _, u := range cellUses(e.p, g, g.Params[k], 0) {
		if u.writes {
			out = append(out, lent.With(u.path...))
		}
	}
	return out
}

// roundTrip: w is the `&mut` borrow behind `mut self` write-back cells, or the store of one back, that only calls in win
// take; callWrites counts what those calls write, so w is no write of its own.
func (e *erasedReaders) roundTrip(w midir.Inst, owner midir.LocalID, win map[ownquery.MovePosition]bool) bool {
	if v, ok := storeValue(w); ok {
		d := e.defs[v]
		if d == nil || d.Op != midir.OpCellTake || len(d.Args) == 0 {
			return false
		}
		_, lent := e.lentCellPlace(d.Args[0], owner)
		return lent && e.spentInCalls(d.Args[0], win)
	}
	if w.Op != midir.OpBorrow || w.Index != 1 {
		return false
	}
	cells := 0
	for l := []midir.LocalID{w.Dst}; len(l) > 0; l = l[1:] {
		for _, p := range e.win.uses[l[0]] {
			switch u := e.useAt(p); {
			case u.Op == midir.OpShare:
				l = append(l, u.Dst)
			case u.Op == midir.OpNewCell && e.spentInCalls(u.Dst, win):
				cells++
			case u.Op != midir.OpDrop:
				return false
			}
		}
	}
	return cells > 0
}

// spentInCalls: every use of write-back cell c is a call in win, its read back, or its drop.
func (e *erasedReaders) spentInCalls(c midir.LocalID, win map[ownquery.MovePosition]bool) bool {
	for _, p := range e.win.uses[c] {
		switch e.useAt(p).Op {
		case midir.OpCall, midir.OpCallValue:
			if !win[p] {
				return false
			}
		case midir.OpCellTake, midir.OpDrop:
		default:
			return false
		}
	}
	return true
}

// useAt is the instruction at p; a terminator's use reads as an instruction of no known kind.
func (e *erasedReaders) useAt(p ownquery.MovePosition) midir.Inst {
	if p.Idx < 0 {
		return midir.Inst{}
	}
	return e.f.Blocks[p.Bid].Insts[p.Idx]
}
