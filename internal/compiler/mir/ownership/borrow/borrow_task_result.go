package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// resultOf is what a call's result may hold: what its callee's summary returns of each argument, all of them for a
// Future (an async callee's too), which keeps its arguments until it runs, or for a callee not known here.
func (r *taskRun) resultOf(st *taskState, cs taskCallees, in *midir.Inst, into srcSet) srcSet {
	if cs.unknown || cs.param || in.Op == midir.OpFutureNew {
		return into
	}
	var out srcSet
	for i := range cs.known {
		ce := &cs.known[i]
		if r.c.p.ByEnt[ce.ent] == nil && r.c.methodOf(ce.ent) == 0 || ownquery.IsAsyncBody(r.c.p.R, ce.ent) {
			return into
		}
		out = out.or(r.mapped(st, ce, in, ce.fate.result))
	}
	return out
}

// mapped turns slot bits of a callee into what the matching arguments hold here; a closure's captures are its value.
func (r *taskRun) mapped(st *taskState, ce *taskCallee, in *midir.Inst, bits srcSet) srcSet {
	n := len(r.c.declaredParams(ce.ent))
	var out srcSet
	bits.each(func(b int) {
		if b >= 2*ce.fate.nslot {
			return
		}
		j := b % ce.fate.nslot
		switch k := ce.off + j; {
		case j >= n && ce.off > 0 && b >= ce.fate.nslot && r.ownsCapture(ce.ent, j-n):
			out = out.or(st.present[in.Args[0]].and(r.borrowBits())).or(r.borrowOf(in.Args[0]))
		case j >= n && ce.off > 0:
			out = out.or(st.present[in.Args[0]])
		case k >= 0 && k < len(in.Args):
			out = out.or(st.present[in.Args[k]])
		}
	})
	return out
}

// startsTask reports a call that may start a Task and hand it back: a declaration without a body that makes one from a
// Future it takes, a body whose summary returns one it started, or a callee not known here whose result may hold one.
func (r *taskRun) startsTask(cs taskCallees, in *midir.Inst) bool {
	t := r.g.Locals[in.Dst].Type
	if cs.unknown && (r.c.p.R.TypeHoldsTask(t) || r.c.p.R.OpenType(t)) {
		return true
	}
	for i := range cs.known {
		ce := &cs.known[i]
		if r.c.p.ByEnt[ce.ent] == nil && r.c.methodOf(ce.ent) == 0 {
			if r.c.startsFromFuture(ce.ent) {
				return true
			}
			continue
		}
		if ce.fate.all || ce.fate.fresh {
			return true
		}
	}
	return false
}

// startedWith is what a started Task may keep: all a body-less starter or an unseen callee is handed, and what each
// known callee's summary says its result holds.
func (r *taskRun) startedWith(st *taskState, cs taskCallees, in *midir.Inst, into srcSet) srcSet {
	out := r.handleBits(into)
	if cs.unknown {
		out = into
	}
	for i := range cs.known {
		ce := &cs.known[i]
		if r.c.p.ByEnt[ce.ent] == nil && r.c.methodOf(ce.ent) == 0 || ce.fate.all {
			return into
		}
		out = out.or(r.mapped(st, ce, in, ce.fate.result))
	}
	return out
}

// ownsCapture reports a closure capture the closure value keeps alive itself, so a borrow of it lasts as that value does.
func (r *taskRun) ownsCapture(cl sem.EntityID, w int) bool {
	if r.c.p.R.Entity(cl).Kind != sem.EntClosure {
		return false
	}
	caps := r.c.p.R.Closure(cl).Captures
	return w < len(caps) && caps[w].Mode != sem.CapBorrow
}
