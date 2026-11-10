package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// taskRun follows g's Task handles and the borrows in them: sources are slot handles, slot borrows (nslot on), g's own
// frame borrow (2*nslot), then started Tasks. A handle dropped before it is joined is lost; a lost Task's contents detach.
type taskRun struct {
	c       *borrowCheck
	g       *midir.Func
	bind    taskBind
	defs    map[midir.LocalID][]*midir.Inst
	sole    map[midir.LocalID]*midir.Inst
	facts   flowSets
	slot    func(midir.LocalID) int
	nslot   int
	ndecl   int
	visible srcSet
	seeds   map[midir.LocalID]srcSet
	cancel  map[int]bool
	srcOf   map[srcKey]int
	calls   map[*midir.Inst]taskCallees
	srcs    []taskSrc
	inside  map[int]srcSet
	// stored is the slots whose places g may leave a Task it started in.
	stored   srcSet
	dropped  srcSet
	detached srcSet
	// returned is what every return has joined; result is what the returned values hold.
	returned srcSet
	returns  bool
	result   srcSet
	events   []taskEvent
	bbits    srcSet
	kinds    []uint8
	places   map[midir.LocalID]placeOf
}

type placeOf struct {
	roots []midir.LocalID
	ok    bool
}

// srcKey names a started Task: the call, and for one left in a place the argument k that lends it (-1 otherwise).
type srcKey struct {
	in *midir.Inst
	k  int
}

type taskSrc struct {
	key   srcKey
	roots []midir.LocalID
}

// taskEvent is an argument a call may leave in a Task nothing joins, with what it holds.
type taskEvent struct {
	in     *midir.Inst
	a      midir.LocalID
	bits   srcSet
	unseen bool
	bound  bool
}

func (c *borrowCheck) newTaskRun(g *midir.Func, bind taskBind) *taskRun {
	declared := c.declaredParams(g.Ent)
	_, slot := c.keptSlots(g, declared)
	r := &taskRun{c: c, g: g, bind: bind, defs: allDefs(g), slot: slot, nslot: len(declared) + len(c.hiddenLocals(g.Ent)), ndecl: len(declared),
		seeds: map[midir.LocalID]srcSet{}, cancel: map[int]bool{}, srcOf: map[srcKey]int{}, inside: map[int]srcSet{},
		calls: map[*midir.Inst]taskCallees{}, kinds: make([]uint8, len(g.Locals)), places: map[midir.LocalID]placeOf{}}
	for i := r.nslot; i <= r.self(); i++ {
		r.bbits = r.bbits.with(i)
	}
	for _, l := range g.Params {
		j := slot(l)
		if j < 0 || j >= r.nslot {
			continue
		}
		var v srcSet
		if (j >= len(declared) || c.takesByValue(g.Ent, j)) && r.holdsHandle(l) {
			v = v.with(j)
		}
		if r.mayBorrow(l) {
			v = v.with(r.nslot + j)
		}
		if r.isRef(l) || g.Locals[l].Cell || c.facts().HoldsBorrow(g.Locals[l].Type) {
			r.visible = r.visible.with(r.nslot + j)
		}
		r.seeds[l] = v
	}
	for bi, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpCancelDone {
				r.cancel[bi] = true
			}
		}
	}
	return r
}

// flow is g's body facts, computed only for a body that needs a callee value resolved.
func (r *taskRun) flow() flowSets {
	if r.facts == nil {
		r.facts = r.c.bodyFacts(r.g)
	}
	return r.facts
}

func (r *taskRun) self() int { return 2 * r.nslot }

func (r *taskRun) borrowBits() srcSet { return r.bbits }

func (r *taskRun) handleBits(v srcSet) srcSet { return v.minus(r.borrowBits()) }

// reportable is what this body knows to be a borrow: of its own frame, or one a parameter visibly brings.
func (r *taskRun) reportable() srcSet { return r.visible.with(r.self()) }

// source is the Task started at key, made on first use.
func (r *taskRun) source(key srcKey, a midir.LocalID) int {
	if s, ok := r.srcOf[key]; ok {
		return s
	}
	s := r.self() + 1 + len(r.srcs)
	r.srcOf[key] = s
	var roots []midir.LocalID
	if key.k >= 0 {
		roots, _ = r.roots(a)
	}
	r.srcs = append(r.srcs, taskSrc{key: key, roots: roots})
	return s
}

func (r *taskRun) run() {
	entry := make([]taskState, len(r.g.Blocks))
	entry[0] = taskState{reached: true, present: map[midir.LocalID]srcSet{}}
	for l, v := range r.seeds {
		if !v.empty() {
			entry[0].present[l] = v
		}
	}
	for changed := true; changed; {
		changed = false
		for bi, blk := range r.g.Blocks {
			if !entry[bi].reached {
				continue
			}
			st := entry[bi].clone()
			for ii := range blk.Insts {
				r.step(&st, bi, &r.g.Blocks[bi].Insts[ii])
			}
			if blk.Term.Op == midir.TermReturn {
				r.noteReturn(st, blk.Term.Args)
			}
			for e, t := range blk.Term.Targets {
				changed = entry[t].merge(r.onEdge(st, blk.Term, e)) || changed
			}
		}
	}
	r.closeDetached()
}

func (r *taskRun) noteReturn(st taskState, args []midir.LocalID) {
	for _, a := range args {
		r.result = r.result.or(st.present[a])
	}
	if !r.returns {
		r.returned, r.returns = st.joined, true
		return
	}
	r.returned = r.returned.and(st.joined)
}

func (r *taskRun) drop(st *taskState, v srcSet) {
	r.dropped = r.dropped.or(r.handleBits(v).minus(st.joined))
}
