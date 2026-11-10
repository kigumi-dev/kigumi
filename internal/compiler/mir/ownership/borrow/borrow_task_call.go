package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

// call hands each argument on by its callee's summary: lost where it may be dropped, detached where it may go into a
// Task nothing joins, joined after a callee that runs it to its end; the result keeps what the summary returns.
func (r *taskRun) call(st *taskState, in *midir.Inst) {
	cs := r.callees(in)
	r.boundEscapes(cs, in)
	var into, joinNow srcSet
	var gone []midir.LocalID
	for _, a := range in.Args {
		into = into.or(st.present[a])
	}
	for k, a := range in.Args {
		v := st.present[a]
		if v.empty() {
			continue
		}
		lost := r.handleBits(v).minus(st.joined)
		consumed := false
		for i := range cs.known {
			ce := &cs.known[i]
			j := k - ce.off
			if j < 0 {
				r.calleeValue(ce, v, lost)
				continue
			}
			f := ce.fate
			byValue := !r.isRef(a) && (j >= len(r.c.declaredParams(ce.ent)) || r.c.takesByValue(ce.ent, j))
			if byValue && (f.all || f.drops.has(j)) {
				r.dropped = r.dropped.or(lost)
			}
			if bits := r.detachBits(ce, j, v); !bits.empty() {
				r.detachArg(in, a, bits, f.all)
			}
			if f.joins.has(j) && !ownquery.IsAsyncBody(r.c.p.R, ce.ent) {
				joinNow = joinNow.or(r.joins(a, 0))
			}
			consumed = consumed || byValue
		}
		if cs.unknown && !r.isRef(a) {
			r.dropped = r.dropped.or(lost)
			if r.c.slotKeepsHandle(r.g, *in, k) {
				r.detachArg(in, a, v, true)
			}
		}
		if consumed {
			gone = append(gone, a)
		}
	}
	if in.Op.Yields() {
		r.assign(st, in.Dst, r.resultOf(st, cs, in, into))
	}
	for k, a := range in.Args {
		if r.c.lendsPlace(r.g, a) {
			r.lendInto(st, a, into)
			if r.storesNew(cs, in, k) {
				r.start(st, srcKey{in, k}, a, into)
			}
		}
	}
	if in.Op.Yields() && r.startsTask(cs, in) {
		r.start(st, srcKey{in, -1}, in.Dst, r.startedWith(st, cs, in, into))
	}
	for _, a := range gone {
		if a != in.Dst {
			delete(st.present, a)
		}
	}
	st.joined = st.joined.or(joinNow)
}

// detachBits is what argument v loses into a Task the callee leaves running: everything for a parameter it detaches,
// the borrow for one whose borrow escapes; a borrow the callee checked on its own already reports stays out.
func (r *taskRun) detachBits(ce *taskCallee, j int, v srcSet) srcSet {
	f := ce.fate
	switch {
	case f.all || f.detaches.has(j):
		return v
	case f.escapes.has(j) && !(f.visible.has(j) && ce.alone().escapes.has(j)):
		return v.and(r.borrowBits())
	}
	return nil
}

// calleeValue applies a closure's summary to what it was built with: its captures share the closure value.
func (r *taskRun) calleeValue(ce *taskCallee, v, lost srcSet) {
	f := ce.fate
	n := len(r.c.declaredParams(ce.ent))
	for j := n; j < n+64; j++ {
		if f.drops.has(j) {
			r.dropped = r.dropped.or(lost)
		}
		r.detached = r.detached.or(r.detachBits(ce, j, v))
	}
}

// boundEscapes takes on a callee that, with the callables this call binds, leaves a Task holding a borrow of its own
// frame running, when the callee checked on its own does not.
func (r *taskRun) boundEscapes(cs taskCallees, in *midir.Inst) {
	for i := range cs.known {
		if ce := &cs.known[i]; ce.fate.escapeSelf && !ce.alone().escapeSelf {
			r.detached = r.detached.with(r.self())
			r.events = append(r.events, taskEvent{in: in, bound: true})
		}
	}
}

func (r *taskRun) detachArg(in *midir.Inst, a midir.LocalID, v srcSet, unseen bool) {
	r.detached = r.detached.or(v)
	r.events = append(r.events, taskEvent{in: in, a: a, bits: v, unseen: unseen})
}

func (r *taskRun) lendInto(st *taskState, a midir.LocalID, v srcSet) {
	if v.empty() {
		return
	}
	roots, ok := r.roots(a)
	if !ok {
		r.drop(st, v)
		return
	}
	r.putInto(st, roots, v)
}

// start begins the Task at key afresh: a handle of it every path here has joined is gone, so nothing still holds it.
func (r *taskRun) start(st *taskState, key srcKey, a midir.LocalID, inside srcSet) {
	s := r.source(key, a)
	one := srcSet{}.with(s)
	if st.joined.has(s) {
		for l, v := range st.present {
			st.present[l] = v.minus(one)
		}
		st.joined = st.joined.minus(one)
	}
	r.inside[s] = r.inside[s].or(inside)
	if key.k < 0 {
		st.present[a] = st.present[a].or(one)
		return
	}
	roots, ok := r.roots(a)
	if !ok {
		r.drop(st, one)
		return
	}
	r.putInto(st, roots, one)
}
