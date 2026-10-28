package mir

// lentCellPlace is the path of owner's place that write-back cell c lends.
func (e *erasedReaders) lentCellPlace(c, owner LocalID) (stepPath, bool) {
	d := e.defs[c]
	if d == nil || d.Op != OpNewCell || len(d.Args) == 0 {
		return nil, false
	}
	b := d.Args[0]
	for d = e.defs[b]; d != nil && d.Op == OpShare && len(d.Args) > 0; d = e.defs[b] {
		b = d.Args[0]
	}
	if d == nil || d.Op != OpBorrow || d.Index != 1 {
		return nil, false
	}
	u, ok := cellAccess(e.f, e.defs, e.win.chained, owner, *d)
	return u.path, ok
}

// callWrites are the parts of owner's place a call may write through the write-back cells it takes: what the callee's
// body writes through that cell parameter, or the whole lent place when no lowered body says.
func (e *erasedReaders) callWrites(in Inst, owner LocalID) []stepPath {
	if in.Op != OpCall && in.Op != OpCallValue {
		return nil
	}
	g := e.callee(in)
	var out []stepPath
	for k, a := range in.Args {
		lent, ok := e.lentCellPlace(a, owner)
		if !ok {
			continue
		}
		if g == nil || !g.Locals[g.Params[k]].Cell {
			out = append(out, lent)
			continue
		}
		for _, u := range cellUses(e.p, g, g.Params[k], 0) {
			if u.writes {
				out = append(out, lent.with(u.path...))
			}
		}
	}
	return out
}

// roundTrip: w is the `&mut` borrow behind `mut self` write-back cells, or the store of one back, that only calls in win
// take; callWrites counts what those calls write, so w is no write of its own.
func (e *erasedReaders) roundTrip(w Inst, owner LocalID, win map[movePos]bool) bool {
	if v, ok := storeValue(w); ok {
		d := e.defs[v]
		if d == nil || d.Op != OpCellTake || len(d.Args) == 0 {
			return false
		}
		_, lent := e.lentCellPlace(d.Args[0], owner)
		return lent && e.spentInCalls(d.Args[0], win)
	}
	if w.Op != OpBorrow || w.Index != 1 {
		return false
	}
	cells := 0
	for l := []LocalID{w.Dst}; len(l) > 0; l = l[1:] {
		for _, p := range e.win.uses[l[0]] {
			switch u := e.useAt(p); {
			case u.Op == OpShare:
				l = append(l, u.Dst)
			case u.Op == OpNewCell && e.spentInCalls(u.Dst, win):
				cells++
			case u.Op != OpDrop:
				return false
			}
		}
	}
	return cells > 0
}

// spentInCalls: every use of write-back cell c is a call in win, its read back, or its drop.
func (e *erasedReaders) spentInCalls(c LocalID, win map[movePos]bool) bool {
	for _, p := range e.win.uses[c] {
		switch e.useAt(p).Op {
		case OpCall, OpCallValue:
			if !win[p] {
				return false
			}
		case OpCellTake, OpDrop:
		default:
			return false
		}
	}
	return true
}

// useAt is the instruction at p; a terminator's use reads as an instruction of no known kind.
func (e *erasedReaders) useAt(p movePos) Inst {
	if p.idx < 0 {
		return Inst{}
	}
	return e.f.Blocks[p.bid].Insts[p.idx]
}
