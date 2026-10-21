package mir

// cellOf is the cell local whose whole content the temporary l holds; field and payload reads of a cell are places of their own.
func (a *owner) cellOf(l LocalID) LocalID {
	if a.cellDefs == nil {
		a.cellDefs = soleDefs(a.f)
	}
	for range 32 {
		if a.f.Locals[l].Ent != 0 {
			return 0
		}
		d := a.cellDefs[l]
		if d == nil || len(d.Args) == 0 {
			return 0
		}
		switch d.Op {
		case OpCellGet:
			if a.cellTracked(d.Args[0]) {
				return d.Args[0]
			}
			return 0
		case OpCopy:
			l = d.Args[0]
		default:
			return 0
		}
	}
	return 0
}

// cellTracked is a move-only local kept in a cell because an immediate lambda writes it; ownership analysis follows its content through st.field.
func (a *owner) cellTracked(c LocalID) bool { return trackedCell(a.p, a.f.Locals[c]) }

func trackedCell(p *Program, loc Local) bool {
	return loc.Ent != 0 && loc.Cell && !loc.Borrowed && !p.R.IsCopy(loc.Type)
}

func hasTrackedCell(p *Program, f *Func) bool {
	for _, l := range f.Locals {
		if trackedCell(p, l) {
			return true
		}
	}
	return false
}

func (a *owner) cellConsume(in *Inst, t LocalID, st *dstate, report bool) {
	if c := a.cellOf(t); c != 0 {
		a.cellRead(in, c, st, report)
		a.checkWhole(in.Node, c, st, report)
		st.field.set(c, ownMoved)
		st.at.set(c, in.Node)
	}
}

func (a *owner) cellStep(in *Inst, st *dstate, report bool) {
	switch in.Op {
	case OpCellSet:
		if c := in.Args[0]; a.cellTracked(c) {
			a.checkGuardHeld(in, c, st, report, 0)
			st.field.del(c)
			a.pl.resetFields(c, st)
		}
		return
	case OpCellGet, OpCellTake:
		if c := in.Args[0]; a.cellTracked(c) {
			a.cellRead(in, c, st, report)
		}
	case OpClosure:
		for i, c := range in.Args {
			switch {
			case !a.cellTracked(c):
			case gone(st.field.get(c)) && a.onlyAssigns(in, i):
				st.field.set(c, ownConflict)
			default:
				a.cellRead(in, c, st, report)
			}
		}
	}
	for i, arg := range in.Args {
		if a.p.consumesArg(in, i) || a.p.primitiveTakes(in, i) || constructs(in.Op) {
			a.cellConsume(in, arg, st, report)
		}
	}
	if in.Op == OpCopy && !in.Guarded && a.f.Locals[in.Dst].Ent != 0 && !a.p.R.IsCopy(a.f.Locals[in.Dst].Type) {
		a.cellConsume(in, in.Args[0], st, report)
	}
}

// moveSource is OpMove's source place; moving a temporary read from a tracked cell consumes the cell's content.
func (a *owner) moveSource(in *Inst, st *dstate, report bool) LocalID {
	a.assignHeld(in, st, report)
	a.cellConsume(in, in.Args[0], st, report)
	return a.pl.canon(in.Args[0])
}

// cellRead reports one use after move per consume of the cell's content, however many reads follow it on one line.
func (a *owner) cellRead(in *Inst, c LocalID, st *dstate, report bool) {
	if report && in.Node != 0 && gone(st.field.get(c)) && a.firstForMove(c, st.at.get(c)) {
		a.check(in, c, st.field.get(c), report)
	}
}

// onlyAssigns: the closure never reads capture i, only stores into it, so creating it reads nothing consumed; whether it runs is unknown, so the cell counts as moved on some paths.
func (a *owner) onlyAssigns(in *Inst, i int) bool {
	cf, ok := a.p.ByEnt[in.Ent]
	if !ok || i >= len(cf.Params) {
		return false
	}
	capt := cf.Params[i]
	for _, blk := range cf.Blocks {
		for _, ci := range blk.Insts {
			for k, arg := range ci.Args {
				if arg == capt && !(ci.Op == OpCellSet && k == 0) && ci.Op != OpDrop {
					return false
				}
			}
		}
		for _, arg := range blk.Term.Args {
			if arg == capt {
				return false
			}
		}
	}
	return true
}

// captureWhole: a closure captures a local whole, so creating it while a field or payload of that local is moved out is a whole use.
func (a *owner) captureWhole(in *Inst, st *dstate, report bool) {
	if a.cellDefs == nil {
		a.cellDefs = soleDefs(a.f)
	}
	for _, arg := range in.Args {
		l := arg
		if d := a.cellDefs[l]; d != nil && (d.Op == OpCopy || d.Op == OpMove) && a.f.Locals[l].Ent == 0 {
			l = d.Args[0]
		}
		if a.f.Locals[l].Ent != 0 {
			a.checkWhole(in.Node, a.pl.canon(l), st, report)
		}
	}
}
