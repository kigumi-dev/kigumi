package borrow

import (
	"maps"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

const (
	lostTaskMsg     = "a spawned task outlives this borrow; await the task before this body ends, or pass it to a call that awaits it"
	detachedTaskMsg = "a task the callee spawns from this argument outlives this borrow; await that task in the callee, or pass a value that holds no borrow"
	unseenTaskMsg   = "the callee is not known here and may spawn this argument into a task that outlives this borrow; pass a value that holds no borrow"
	placeTaskMsg    = "a task this call may start in the place it lends outlives this borrow; await the task before this body ends"
	boundTaskMsg    = "with the callables this call passes, the callee leaves a task running that holds a borrow; await the task in the callee, or pass callables that do not start one"
)

// checkTaskJoins rejects a borrow of this body that goes into a Task nothing joins: one this body starts and loses, or
// one a callee starts and leaves running.
func (c *borrowCheck) checkTaskJoins() {
	if !c.tasksStart() {
		return
	}
	r := c.newTaskRun(c.f, nil)
	r.run()
	keep := r.reportable()
	for i, src := range r.srcs {
		s := r.self() + 1 + i
		if !r.lost(s) || r.within(s).and(keep).empty() {
			continue
		}
		if src.key.k < 0 {
			c.report(src.key.in.Node, src.key.in.Dst, "borrow-escape", lostTaskMsg)
		} else {
			c.report(src.key.in.Node, src.key.in.Dst, "borrow-escape", placeTaskMsg)
		}
	}
	for _, ev := range r.events {
		switch {
		case ev.bound:
			c.report(ev.in.Node, ev.in.Dst, "borrow-escape", boundTaskMsg)
		case r.reach(ev.bits).and(keep).empty():
		case ev.unseen:
			c.report(ev.in.Node, ev.a, "borrow-escape", unseenTaskMsg)
		default:
			c.report(ev.in.Node, ev.a, "borrow-escape", detachedTaskMsg)
		}
	}
}

// reach is bits with what the Tasks among them hold.
func (r *taskRun) reach(bits srcSet) srcSet {
	out := bits
	bits.each(func(s int) {
		if s > r.self() {
			out = out.or(r.within(s))
		}
	})
	return out
}

// tasksStart reports whether a lowered body names a declaration that starts a Task from a Future: with none, no Task
// exists for a body to lose. A body lowered later can still turn the answer to yes.
func (c *borrowCheck) tasksStart() bool {
	st := c.state
	var funcs []*midir.Func
	if c.p.Funcs != nil {
		funcs = c.p.Funcs()
	} else {
		funcs = slices.Collect(maps.Values(c.p.ByEnt))
	}
	if st.taskStarts == 2 || st.taskStarts == 1 && st.taskStartsSeen == len(funcs) {
		return st.taskStarts == 2
	}
	st.taskStarts, st.taskStartsSeen = 1, len(funcs)
	for _, f := range funcs {
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Ent != 0 && c.startsTaskByDecl(in.Ent) {
					st.taskStarts = 2
					return true
				}
			}
		}
	}
	return false
}

func (c *borrowCheck) startsTaskByDecl(e sem.EntityID) bool {
	if v, ok := c.state.taskStarters[e]; ok {
		return v
	}
	v := false
	if c.p.R.Entity(e).Kind == sem.EntFn && c.p.R.Fn(e).Body == 0 && c.p.R.TypeHoldsTask(c.p.R.Types.Node(c.p.R.Fn(e).Sig).Elem) {
		for _, q := range c.declaredParams(e) {
			if n := c.p.R.Types.Node(c.p.R.Entity(q).Type); n.Kind == sem.KNamed && n.Ent == c.p.R.Types.FutureEnt() {
				v = true
			}
		}
	}
	if c.state.taskStarters == nil {
		c.state.taskStarters = map[sem.EntityID]bool{}
	}
	c.state.taskStarters[e] = v
	return v
}
