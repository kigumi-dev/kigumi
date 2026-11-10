package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// taskFate summarizes a body under one binding of its callable slots, per slot: drop unjoined, detach into an unjoined
// Task, let its borrow escape into one, join on every return, or store a started Task in its place.
type taskFate struct {
	drops, detaches, escapes, joins, stores srcSet
	// visible is the slots whose borrow the body's own check sees; result is what its returned values hold, by slot,
	// and fresh says they may hold a Task it started.
	visible    srcSet
	result     srcSet
	fresh      bool
	escapeSelf bool
	nslot      int
	all        bool
}

func (c *borrowCheck) summarizeTasks(g *midir.Func, bind taskBind) *taskFate {
	r := c.newTaskRun(g, bind)
	r.run()
	f := &taskFate{nslot: r.nslot, stores: r.stored, escapeSelf: r.detached.has(r.self()), fresh: r.result.anyFrom(r.self() + 1)}
	for j := range r.nslot {
		if r.visible.has(r.nslot + j) {
			f.visible = f.visible.with(j)
		}
		if r.lost(j) {
			f.drops = f.drops.with(j)
		}
		if r.detached.has(j) {
			f.detaches = f.detaches.with(j)
		}
		if r.detached.has(r.nslot + j) {
			f.escapes = f.escapes.with(j)
		}
		if r.returns && r.returned.has(j) {
			f.joins = f.joins.with(j)
		}
	}
	r.result.each(func(b int) {
		if b < 2*r.nslot {
			f.result = f.result.with(b)
		}
	})
	return f
}

func (f *taskFate) equal(o *taskFate) bool {
	return f.all == o.all && f.fresh == o.fresh && f.escapeSelf == o.escapeSelf && f.drops.equal(o.drops) &&
		f.detaches.equal(o.detaches) && f.escapes.equal(o.escapes) && f.joins.equal(o.joins) && f.stores.equal(o.stores) &&
		f.result.equal(o.result)
}

func allBits(n int) srcSet {
	var s srcSet
	for j := range n {
		s = s.with(j)
	}
	return s
}

// signatureFate reads a declaration without a body by its signature: it keeps what it is handed, runs a `Future[T]`
// to its end when it returns that `T`, and hands back all of it.
func (c *borrowCheck) signatureFate(e sem.EntityID) *taskFate {
	if c.p.R.Entity(e).Kind != sem.EntFn {
		return &taskFate{all: true}
	}
	params := c.declaredParams(e)
	f := &taskFate{nslot: len(params)}
	for j, q := range params {
		if c.runsToResult(e, q) {
			f.joins = f.joins.with(j)
		}
		f.result = f.result.with(j).with(len(params) + j)
	}
	return f
}

func (c *borrowCheck) runsToResult(e, q sem.EntityID) bool {
	tt := c.p.R.Types
	ret := tt.Node(c.p.R.Fn(e).Sig).Elem
	n := tt.Node(c.p.R.Entity(q).Type)
	return tt.Kind(ret) == sem.KParam && n.Kind == sem.KNamed && n.Ent == tt.FutureEnt() && len(n.Args) == 1 && n.Args[0] == ret
}

// startsFromFuture reports a declaration without a body that starts the Future it takes as the Task it returns.
func (c *borrowCheck) startsFromFuture(e sem.EntityID) bool {
	for _, q := range c.declaredParams(e) {
		if c.runsFuture(e, q) {
			return true
		}
	}
	return false
}

// runsFuture reports a Future parameter a bodiless declaration starts as the task it returns.
func (c *borrowCheck) runsFuture(e, q sem.EntityID) bool {
	n := c.p.R.Types.Node(c.p.R.Entity(q).Type)
	return c.p.R.Entity(e).Kind == sem.EntFn && c.p.ByEnt[e] == nil && n.Kind == sem.KNamed && n.Ent == c.p.R.Types.FutureEnt() &&
		c.p.R.TypeHoldsTask(c.p.R.Types.Node(c.p.R.Fn(e).Sig).Elem)
}
