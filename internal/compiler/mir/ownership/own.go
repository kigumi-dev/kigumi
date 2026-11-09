package ownership

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/syntax"
)

// Analyze runs a forward dataflow over the CFG: every use needs a
// valid local, moves consume, drops of moved locals are removed, and
// joins must agree. A field moved out of an owned place
// keeps its own presence state independent of the place that held it;
// using that place as a whole while one of its fields is gone is also an
// error, and a drop of it becomes a drop of the fields still there.
func Analyze(p *Input, f *midir.Func, ghosts []GhostRun) ([]MoveEvent, bool) {
	moveOnly := func(l midir.Local) bool {
		return !p.R.IsCopy(l.Type) && !l.Cell
	}
	tracked := map[midir.LocalID]bool{}
	for i, l := range f.Locals {
		if l.Ent != 0 && moveOnly(l) {
			tracked[midir.LocalID(i)] = true
		}
	}
	pl := findPlaces(p, f)
	for l := range pl.key {
		if moveOnly(f.Locals[l]) {
			tracked[l] = true
		}
	}
	if len(tracked) == 0 && !hasTrackedCell(p, f) {
		return nil, false
	}
	a := &owner{p: p, f: f, ghosts: ghosts, pl: pl, tracked: tracked, in: make([]*dstate, len(f.Blocks)),
		reported: map[syntax.NodeID]bool{}, defNode: map[midir.LocalID]syntax.NodeID{}, thread: mirflow.NewFlagThread(f)}
	entry := newDstate()
	for _, prm := range f.Params {
		if tracked[prm] {
			entry.val.set(prm, ownValid)
		}
	}
	a.in[0] = entry
	a.solve(0)
	a.solveGhosts()
	for bid := range f.Blocks {
		if a.in[bid] != nil {
			a.transfer(midir.BlockID(bid), cloneD(a.in[bid]), true)
		}
	}
	return a.moves, true
}

func (a *owner) solve(start midir.BlockID) {
	work := []midir.BlockID{start}
	for len(work) > 0 {
		bid := work[0]
		work = work[1:]
		out := a.transfer(bid, cloneD(a.in[bid]), false)
		if a.thread.AllThreaded(bid) {
			continue
		}
		for _, succ := range a.f.Blocks[bid].Term.Targets {
			if a.merge(succ, out) {
				work = append(work, succ)
			}
			if to, ok := a.thread.Target(bid, succ); ok && a.merge(to, a.transfer(succ, cloneD(out), false)) {
				work = append(work, to)
			}
		}
	}
}

// transfer walks a block; in report mode it emits diagnostics and rewrites
// drops of consumed locals into no-ops or, of a partially moved place,
// into drops of the fields still there.
func (a *owner) transfer(bid midir.BlockID, st *dstate, report bool) *dstate {
	blk := &a.f.Blocks[bid]
	a.cur = st
	for i := range blk.Insts {
		in := &blk.Insts[i]
		a.pos = MovePosition{Bid: bid, Idx: i}
		if in.Op.Yields() {
			a.defNode[in.Dst] = in.Node
		}
		switch in.Op {
		case midir.OpDrop:
			l := a.pl.canon(in.Args[0])
			if !a.tracked[l] {
				continue
			}
			// A value the local may or may not hold keeps its drop: the
			// backends store null into every local first and into a local a
			// move emptied, so dropping an absent value is a no-op. Only a
			// drop that can never find a value goes.
			if st.val.get(l) == ownUninit || st.val.get(l) == ownMoved {
				if report {
					in.Op = midir.OpNop
					in.Args = nil
				}
				continue
			}
			// A partially moved place still gets a plain drop: every
			// backend's free already walks an aggregate's fields and skips
			// a moved-out (null) one, so the fields still there are
			// released the same way a whole value's would be.
			in.Emptied = report && a.emptied(l, st)
			in.Partial = report && a.partial(l, st)
			st.val.set(l, ownUninit)
			continue
		case midir.OpMove:
			a.moveInst(in, st, report)
		case midir.OpNop:
			a.guardedMove(in, st, report)
			continue
		default:
			a.onceCall(in, st, report)
			a.useArgs(in, st, report)
		}
		if in.Op == midir.OpField || in.Op == midir.OpFieldMove || in.Op == midir.OpPayload {
			if l := a.pl.canon(in.Dst); a.tracked[l] {
				a.check(in, l, st.val.get(l), report)
				a.check(in, l, st.field.get(l), report)
				if in.Op == midir.OpFieldMove {
					a.fieldMove(in, l, st, report)
				}
			}
		}
		if in.Op == midir.OpSetField {
			if l, ok := a.pl.stored(a.f, *in); ok {
				st.val.set(l, ownValid)
				st.field.set(l, ownValid)
				st.called.del(l)
				a.pl.resetFields(l, st)
			}
		}
		if a.pl.isPlace(in.Dst) {
			if l := a.pl.canon(in.Dst); st.val.get(l) == ownUninit {
				st.val.set(l, ownValid)
			}
			continue
		}
		if in.Op != midir.OpDrop && in.Op != midir.OpSetField && in.Op != midir.OpSetIndex && in.Op != midir.OpCellSet && in.Op != midir.OpGlobalSet && in.Op != midir.OpFutureDone && in.Op != midir.OpCancelDone && a.tracked[in.Dst] {
			st.val.set(in.Dst, ownValid)
			st.called.del(in.Dst)
			a.pl.resetFields(in.Dst, st)
		}
	}
	if blk.Term.Op == midir.TermReturn {
		for _, arg := range blk.Term.Args {
			l := a.pl.canon(arg)
			if a.tracked[l] {
				a.check(nil, l, st.val.get(l), report)
				a.checkWhole(a.defNode[arg], l, st, report)
				if a.pl.isPlace(arg) {
					a.checkDropOwner(a.defNode[arg], l, report)
				}
			}
		}
		a.checkBorrowedWhole(st, report)
	}
	return st
}
