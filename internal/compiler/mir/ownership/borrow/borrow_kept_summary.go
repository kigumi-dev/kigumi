package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// keptSummary is what a body may keep in its result and in each `&mut` / `mut self` parameter's place: bit i for
// parameter i, bit contentShift+i for what its referent held at the call; all stands for a body it could not follow.
type keptSummary struct {
	result uint64
	stores map[int]uint64
	all    bool
}

const contentShift = 32

func (s *keptSummary) keepsResult(i int) bool { return s.all || s.result&(1<<i) != 0 }

func (s *keptSummary) movesOut(i int) bool { return s.all || s.result&(1<<(i+contentShift)) != 0 }

func (s *keptSummary) storesInto(t, i int) bool { return s.all || s.stores[t]&(1<<i) != 0 }

func (s *keptSummary) movesInto(t, i int) bool {
	return s.all || s.stores[t]&(1<<(i+contentShift)) != 0
}

// keptKey names a summary of g with its callable parameters bound to the fn items a call site passes.
type keptKey struct {
	g    *midir.Func
	bind string
}

func (c *borrowCheck) keptOf(e sem.EntityID) *keptSummary { return c.keptBound(e, nil) }

func (c *borrowCheck) keptBound(e sem.EntityID, bind map[int][]sem.EntityID) *keptSummary {
	if ownquery.IsAsyncBody(c.p.R, e) {
		return nil
	}
	return c.keptStoreSummary(e, bind)
}

func (c *borrowCheck) keptStoreSummary(e sem.EntityID, bind map[int][]sem.EntityID) *keptSummary {
	g := c.p.ByEnt[e]
	if g == nil {
		return nil
	}
	key := keptKey{g: g, bind: bindKey(bind)}
	m := c.offloads()
	if s, ok := m.keeps[key]; ok {
		return s
	}
	m.keeps[key] = &keptSummary{all: true}
	m.keeps[key] = c.keptBody(g, bind)
	return m.keeps[key]
}

// keptCtx is one keptBody run over g: the facts so far, and index, a local's declared parameter index or -1.
type keptCtx struct {
	g     *midir.Func
	defs  map[midir.LocalID][]*midir.Inst
	facts map[midir.LocalID]uint64
	bind  map[int][]sem.EntityID
	index func(midir.LocalID) int
	put   func(midir.LocalID, uint64)
}

// keptBody follows each parameter through g only into locals that can keep a borrow or are a place g borrows, so a value
// computed off it (a length) or moved out of its referent (keptCall) does not count as keeping it.
func (c *borrowCheck) keptBody(g *midir.Func, bind map[int][]sem.EntityID) *keptSummary {
	declared := c.declaredParams(g.Ent)
	if len(declared) > contentShift {
		return &keptSummary{all: true}
	}
	index := func(l midir.LocalID) int {
		if e := g.Locals[l].Ent; e != 0 && slices.Contains(g.Params, l) {
			return slices.Index(declared, e)
		}
		return -1
	}
	facts := map[midir.LocalID]uint64{}
	for _, l := range g.Params {
		if i := index(l); i >= 0 {
			facts[l] |= 1 << i
		}
	}
	k := &keptCtx{g: g, defs: allDefs(g), facts: facts, bind: bind, index: index}
	places := borrowedPlaces(g, k.defs)
	for changed := true; changed; {
		changed = false
		k.put = func(l midir.LocalID, f uint64) {
			t := g.Locals[l].Type
			if facts[l]|f == facts[l] || !places[l] && !g.Locals[l].Cell && !c.facts().HoldsBorrow(t) && !c.holdsDeferred(t) && !c.facts().NamesTypeParam(t) {
				return
			}
			facts[l] |= f
			changed = true
		}
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				c.keptStep(k, in)
			}
		}
	}
	s := &keptSummary{stores: map[int]uint64{}}
	for _, blk := range g.Blocks {
		for _, a := range blk.Term.Args {
			if blk.Term.Op == midir.TermReturn {
				s.result |= facts[a]
			}
		}
	}
	for _, l := range g.Params {
		if i := index(l); i >= 0 {
			s.stores[i] = facts[l] &^ (1<<i | 1<<(i+contentShift))
		}
	}
	return s
}

func (c *borrowCheck) own(k *keptCtx, a midir.LocalID) uint64 {
	var bits uint64
	for _, r := range c.refRoots(k.g, k.defs, a) {
		if i := k.index(r); i >= 0 {
			bits |= 1 << i
		}
	}
	return bits
}

// moved is what a callee taking something out of lent operand a may hand on: what a's place was given in this body and
// what the referents of the parameters a names (own) held at the call, not those parameters themselves.
func (c *borrowCheck) moved(k *keptCtx, a midir.LocalID) uint64 {
	t := k.g.Locals[a].Type
	if c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	if !c.facts().HoldsBorrow(t) && !c.holdsDeferred(t) && !c.facts().NamesTypeParam(t) && !c.hidesLayout(t) {
		return 0
	}
	own := c.own(k, a)
	return k.facts[a]&^own | own<<contentShift
}

func (c *borrowCheck) keptStep(k *keptCtx, in midir.Inst) {
	g := k.g
	into := func(a midir.LocalID, f uint64) {
		roots, ok := c.placeRoots(g, k.defs, a)
		if !ok {
			roots = slices.DeleteFunc(slices.Clone(g.Params), func(p midir.LocalID) bool { return !c.lendsPlace(g, p) })
		}
		for _, r := range roots {
			k.put(r, f)
		}
	}
	switch bodyRoles[in.Op] {
	case roleCarry:
		var from uint64
		for _, a := range in.Args {
			from |= k.facts[a]
		}
		k.put(in.Dst, from)
	case roleStore:
		if in.Op != midir.OpGlobalSet {
			into(in.Args[0], k.facts[in.Args[len(in.Args)-1]])
		}
	case roleCall:
		res, lent := c.keptCall(k, in)
		if in.Op.Yields() {
			k.put(in.Dst, res)
		}
		for i, a := range in.Args {
			if c.lendsPlace(g, a) || c.mayHoldPlace(g, a) {
				into(a, lent(i))
			}
		}
	}
}
