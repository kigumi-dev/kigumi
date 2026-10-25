package mir

import (
	"slices"

	"kigumi/internal/sem"
)

type paramKey struct {
	g *Func
	i int
}

// path is a prefix, empty for the whole parameter; beyond lies past a reference the place holds, leaving its fields.
type paramStore struct {
	path   stepPath
	beyond bool
}

// siteEntry is a callee summary; from is kept while its call-graph component is open in siteSCC.
type siteEntry struct {
	sites []paramStore
	from  map[LocalID]derivation
}

// derives lists the opcodes whose result reaches what an operand points at: copies, cells, borrows and projections.
var derives = map[Opcode]bool{OpCopy: true, OpMove: true, OpAlias: true, OpShare: true, OpNewCell: true, OpCellGet: true, OpCellTake: true,
	OpBorrow: true, OpField: true, OpFieldMove: true, OpPayload: true, OpIndex: true, OpUnbox: true}

func (c *borrowCheck) paramStores(g *Func, i int) bool {
	return len(c.paramStoreSites(g, i)) > 0
}

// paramStoreSites: a write into a place under the parameter, or a hand-off of a `&mut` from it to code that may store.
// Mutually recursive summaries settle together as one strongly connected component (Tarjan), iterated to a fixpoint.
func (c *borrowCheck) paramStoreSites(g *Func, i int) []paramStore {
	if g == nil || i >= len(g.Params) {
		return []paramStore{{}}
	}
	key := paramKey{g, i}
	if e, ok := c.storeMemo[key]; ok {
		if slot, open := c.siteSCC.slot(key); open {
			c.siteLow = min(c.siteLow, slot)
		}
		return e.sites
	}
	if c.storeMemo == nil {
		c.storeMemo = map[paramKey]*siteEntry{}
	}
	e := &siteEntry{from: c.paramDerivations(g, g.Params[i])}
	c.storeMemo[key] = e
	slot := c.siteSCC.push(key)
	outer := c.siteLow
	c.siteLow = noCut
	e.sites = c.collectStoreSites(g, e.from)
	low := c.siteLow
	if low < slot {
		c.siteLow = min(outer, low)
		return e.sites
	}
	c.settleSites(slot, low == slot)
	c.siteLow = outer
	return e.sites
}

// settleSites closes the component open from slot up, recomputing its members until none changes when they recurse;
// the latest opened first, since each was opened by a caller below it that reads it.
func (c *borrowCheck) settleSites(slot int, recursive bool) {
	members := c.siteSCC.members(slot)
	for round := 0; recursive; round++ {
		changed := false
		for _, k := range slices.Backward(members) {
			m := c.storeMemo[k]
			if s := c.collectStoreSites(k.g, m.from); !sameSites(s, m.sites) {
				m.sites, changed = s, true
			}
		}
		if !changed {
			break
		}
		if round == 64 {
			for _, k := range members {
				c.storeMemo[k].sites = []paramStore{{}}
			}
			break
		}
	}
	for _, k := range c.siteSCC.pop(slot) {
		c.storeMemo[k].from = nil
	}
}

func (c *borrowCheck) collectStoreSites(g *Func, from map[LocalID]derivation) []paramStore {
	c.siteRuns++
	holdsMut := func(l LocalID) bool {
		_, ok := from[l]
		return ok && c.typeHolds(g.Locals[l].Type, func(t sem.TypeID) bool { _, mut := c.p.R.MutRefElem(t); return mut })
	}
	var out []paramStore
	add := func(d derivation, below paramStore) {
		s := paramStore{path: d.path, beyond: d.beyond || below.beyond}
		if !d.cut {
			s.path = d.path.with(below.path...)
		}
		if !slices.ContainsFunc(out, func(o paramStore) bool { return o.beyond == s.beyond && slices.Equal(o.path, s.path) }) {
			out = append(out, s)
		}
	}
	for _, blk := range g.Blocks {
		for _, a := range blk.Term.Args {
			if holdsMut(a) {
				add(from[a], paramStore{})
			}
		}
		for _, in := range blk.Insts {
			c.instStoreSites(g, in, from, holdsMut, add)
		}
	}
	return out
}

func sameSites(a, b []paramStore) bool {
	return len(a) == len(b) && !slices.ContainsFunc(a, func(s paramStore) bool {
		return !slices.ContainsFunc(b, func(o paramStore) bool { return o.beyond == s.beyond && slices.Equal(o.path, s.path) })
	})
}

// instStoreSites adds the stores in may make through a value reached from the parameter: a write of a tracked value
// under it, or a `&mut` from it handed on to code that may store, or put anywhere else.
func (c *borrowCheck) instStoreSites(g *Func, in Inst, from map[LocalID]derivation, holdsMut func(LocalID) bool, add func(derivation, paramStore)) {
	switch {
	case in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet:
		v := in.Args[len(in.Args)-1]
		if d, ok := from[in.Args[0]]; ok && c.carries(g.Locals[v].Type) {
			add(d, paramStore{})
		}
		if holdsMut(v) {
			add(from[v], paramStore{})
		}
		return
	case derives[in.Op], in.Op == OpDrop, in.Op == OpRelease:
		return
	}
	for k, a := range in.Args {
		if !holdsMut(a) {
			continue
		}
		if h := c.p.ByEnt[in.Ent]; in.Op == OpClosure || in.Op == OpCall && h != nil && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
			for _, s := range c.paramStoreSites(h, k) {
				add(from[a], s)
			}
			continue
		}
		// A callee without a body here can only store what it is handed, and an index cannot hold a closure.
		other := func(b LocalID) bool { return b != a && c.carries(g.Locals[b].Type) }
		if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpBuiltin || slices.ContainsFunc(in.Args, other) {
			add(from[a], paramStore{})
		}
	}
}
