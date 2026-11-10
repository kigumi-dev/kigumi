package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (r *taskRun) step(st *taskState, bi int, in *midir.Inst) {
	switch in.Op {
	case midir.OpDrop, midir.OpRelease:
		// The cancellation path ends the task, which cancels and waits for every child before it finishes.
		if a := in.Args[0]; !in.Scoped && !in.Emptied && !r.isRef(a) && !r.cancel[bi] {
			r.drop(st, st.present[a])
			delete(st.present, a)
		}
	case midir.OpMove:
		r.assign(st, in.Dst, st.present[in.Args[0]])
		delete(st.present, in.Args[0])
	case midir.OpBorrow:
		r.assign(st, in.Dst, st.present[in.Args[0]].and(r.borrowBits()).or(r.borrowOf(in.Args[0])))
	case midir.OpClosure:
		r.assign(st, in.Dst, r.captured(st, in))
	case midir.OpAwait:
		a := in.Args[0]
		st.joined = st.joined.or(r.joins(a, 0))
		r.assign(st, in.Dst, st.present[a].minus(st.joined))
		delete(st.present, a)
	case midir.OpGlobalSet:
		r.drop(st, st.present[in.Args[0]])
	case midir.OpSetField, midir.OpSetIndex, midir.OpCellSet:
		r.store(st, in)
	case midir.OpCall, midir.OpCallValue, midir.OpFutureNew, midir.OpCallC, midir.OpAsm:
		r.call(st, in)
	default:
		if bodyRoles[in.Op] == roleCarry || in.Op == midir.OpBuiltin {
			var v srcSet
			for _, a := range in.Args {
				v = v.or(st.present[a]).or(r.through(st, a))
			}
			r.assign(st, in.Dst, v)
		}
	}
}

// borrowOf is the borrow a reference to the place of l takes: what a parameter slot lends or a closure captured, or
// this frame, which owns a by-value parameter.
func (r *taskRun) borrowOf(l midir.LocalID) srcSet {
	roots, _ := r.roots(l)
	var v srcSet
	for _, root := range roots {
		switch j := r.slot(root); {
		case j >= 0 && j < r.nslot && (r.isRef(root) || r.g.Locals[root].Cell || j >= r.ndecl):
			v = v.with(r.nslot + j)
		case r.g.Locals[root].Ent != 0 || r.g.Locals[root].Cell || !r.projected(root):
			v = v.with(r.self())
		}
	}
	return v
}

// projected reports a temporary read out of, or aliasing, another place, whose borrow is that place's.
func (r *taskRun) projected(l midir.LocalID) bool {
	for _, d := range r.defs[l] {
		switch d.Op {
		case midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpPayload, midir.OpCellGet, midir.OpAlias, midir.OpGlobalGet:
			return true
		case midir.OpCopy, midir.OpMove, midir.OpShare:
			if r.isRef(l) {
				return true
			}
		}
	}
	return false
}

// captured is what a closure literal holds: what it captures, and the borrow of what it only borrows; a cell it shares
// keeps its own count.
func (r *taskRun) captured(st *taskState, in *midir.Inst) srcSet {
	var caps []sem.Capture
	if r.c.p.R.Entity(in.Ent).Kind == sem.EntClosure {
		caps = r.c.p.R.Closure(in.Ent).Captures
	}
	var v srcSet
	for j, a := range in.Args {
		v = v.or(st.present[a])
		if caps != nil && j < len(caps) && caps[j].Mode == sem.CapBorrow {
			v = v.or(r.borrowOf(a))
		}
	}
	return v
}

// through is what a value read through reference a may hold: what the places it reaches hold, as a `&mut` argument's
// write-back reads it.
func (r *taskRun) through(st *taskState, a midir.LocalID) srcSet {
	if !r.isRef(a) {
		return nil
	}
	roots, _ := r.roots(a)
	var v srcSet
	for _, root := range roots {
		if root != a {
			v = v.or(st.present[root])
		}
	}
	return v
}

// store loses a value put into a place this body cannot name, and the handle a handle overwrites.
func (r *taskRun) store(st *taskState, in *midir.Inst) {
	val := in.Args[len(in.Args)-1]
	v := st.present[val]
	roots, ok := r.roots(in.Args[0])
	if !ok {
		r.drop(st, v)
		return
	}
	if r.c.typeRuns(r.g.Locals[val].Type, runsHandle) {
		for _, root := range roots {
			r.drop(st, st.present[root])
		}
	}
	r.putInto(st, roots, v)
}

// putInto puts v into each place root; a slot's place notes a Task this body started.
func (r *taskRun) putInto(st *taskState, roots []midir.LocalID, v srcSet) {
	fresh := v.anyFrom(r.self() + 1)
	for _, root := range roots {
		if j := r.slot(root); j >= 0 && j < r.nslot && r.isRef(root) && fresh {
			r.stored = r.stored.with(j)
		}
		r.add(st, root, v)
	}
}
