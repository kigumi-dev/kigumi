package mir

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

// cellOf is the cell local whose whole content the temporary l holds; field and payload reads of a cell are places of their own.
func (a *owner) cellOf(l midir.LocalID) (midir.LocalID, bool) {
	if a.cellDefs == nil {
		a.cellDefs = mirflow.SoleDefs(a.f)
	}
	for range 32 {
		if a.f.Locals[l].Ent != 0 {
			return 0, false
		}
		d := a.cellDefs[l]
		if d == nil || len(d.Args) == 0 {
			return 0, false
		}
		switch d.Op {
		case midir.OpCellGet:
			return d.Args[0], a.cellTracked(d.Args[0])
		case midir.OpCopy:
			l = d.Args[0]
		default:
			return 0, false
		}
	}
	return 0, false
}

// cellTracked is a move-only local kept in a cell because an immediate lambda writes it; ownership analysis follows its content through st.field.
func (a *owner) cellTracked(c midir.LocalID) bool { return trackedCell(a.p, a.f.Locals[c]) }

func trackedCell(p *ownquery.Input, loc midir.Local) bool {
	return loc.Ent != 0 && loc.Cell && !loc.Borrowed && !p.R.IsCopy(loc.Type)
}

func hasTrackedCell(p *ownquery.Input, f *midir.Func) bool {
	for _, l := range f.Locals {
		if trackedCell(p, l) {
			return true
		}
	}
	return false
}

func (a *owner) cellConsume(in *midir.Inst, t midir.LocalID, st *dstate, report bool) {
	if c, ok := a.cellOf(t); ok {
		a.cellRead(in, c, st, report)
		a.checkWhole(in.Node, c, st, report)
		st.field.set(c, ownMoved)
		st.at.set(c, in.Node)
	}
}

func (a *owner) cellStep(in *midir.Inst, st *dstate, report bool) {
	switch in.Op {
	case midir.OpCellSet:
		if c := in.Args[0]; a.cellTracked(c) {
			a.checkGuardHeld(in, c, st, report, 0)
			st.field.del(c)
			a.pl.resetFields(c, st)
		}
		return
	case midir.OpCellGet, midir.OpCellTake:
		if c := in.Args[0]; a.cellTracked(c) {
			a.cellRead(in, c, st, report)
		}
	case midir.OpClosure:
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
		if ownquery.ConsumesArg(a.p.R, a.p.ByEnt, in, i) || ownquery.PrimitiveTakes(a.p.R, in, i) || ownquery.Constructs(in.Op) {
			a.cellConsume(in, arg, st, report)
		}
	}
	if in.Op == midir.OpCopy && !in.Guarded && a.f.Locals[in.Dst].Ent != 0 && !a.p.R.IsCopy(a.f.Locals[in.Dst].Type) {
		a.cellConsume(in, in.Args[0], st, report)
	}
}

// moveSource is OpMove's source place; moving a temporary read from a tracked cell consumes the cell's content.
func (a *owner) moveSource(in *midir.Inst, st *dstate, report bool) midir.LocalID {
	a.assignHeld(in, st, report)
	a.cellConsume(in, in.Args[0], st, report)
	return a.pl.canon(in.Args[0])
}

// cellRead reports one use after move per consume of the cell's content, however many reads follow it on one line.
func (a *owner) cellRead(in *midir.Inst, c midir.LocalID, st *dstate, report bool) {
	if report && in.Node != 0 && gone(st.field.get(c)) && a.firstForMove(c, st.at.get(c)) {
		a.check(in, c, st.field.get(c), report)
	}
}

// onlyAssigns: the closure never reads capture i, only stores into it, so creating it reads nothing consumed; whether it runs is unknown, so the cell counts as moved on some paths.
func (a *owner) onlyAssigns(in *midir.Inst, i int) bool {
	cf, ok := a.p.ByEnt[in.Ent]
	if !ok || i >= len(cf.Params) {
		return false
	}
	capt := cf.Params[i]
	for _, blk := range cf.Blocks {
		for _, ci := range blk.Insts {
			for k, arg := range ci.Args {
				if arg == capt && !(ci.Op == midir.OpCellSet && k == 0) && ci.Op != midir.OpDrop {
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
func (a *owner) captureWhole(in *midir.Inst, st *dstate, report bool) {
	if a.cellDefs == nil {
		a.cellDefs = mirflow.SoleDefs(a.f)
	}
	for _, arg := range in.Args {
		l := arg
		if d := a.cellDefs[l]; d != nil && (d.Op == midir.OpCopy || d.Op == midir.OpMove) && a.f.Locals[l].Ent == 0 {
			l = d.Args[0]
		}
		if a.f.Locals[l].Ent != 0 {
			a.checkWhole(in.Node, a.pl.canon(l), st, report)
		}
	}
}
