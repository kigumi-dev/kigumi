package mir

const guardFailed = "guard-failed"

func (b *builder) releaseGuardMoves(moves []guardMove) {
	for _, m := range moves {
		if m.take == nil {
			b.emit(Inst{Op: OpNop, Args: []LocalID{m.place}, Node: m.node, Guarded: true, Str: guardFailed})
		}
	}
}

func (b *builder) markGuardMoves(moves []guardMove) {
	for _, m := range moves {
		if m.take != nil {
			b.takeScrutinee(m.place, m.take, true)
			continue
		}
		if b.wholeLocal(m.place) {
			// The binding holds its own reference; the passed guard empties the local it was copied from.
			t := b.emit(Inst{Op: OpMove, Dst: b.temp(b.fn.Locals[m.place].Type), Args: []LocalID{m.place}, Node: m.node, Guarded: true})
			b.emit(Inst{Op: OpDrop, Args: []LocalID{t}, Node: m.node})
			continue
		}
		b.emit(Inst{Op: OpNop, Args: []LocalID{m.place, m.bind}, Node: m.node, Guarded: true})
	}
}

func (b *builder) wholeLocal(v LocalID) bool {
	l := b.fn.Locals[v]
	return l.Ent != 0 && !l.Cell && !l.Borrowed && !b.shared[v] && !b.r.IsCopy(l.Type)
}

// readsCell is a scrutinee temporary read out of a move-only cell local; its guarded binding takes the cell's content once the guard passes.
func (b *builder) readsCell(v LocalID) bool {
	for _, blk := range b.fn.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpCellGet && in.Dst == v {
				return trackedCell(b.p, b.fn.Locals[in.Args[0]])
			}
		}
	}
	return false
}
