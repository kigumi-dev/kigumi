package mir

import "slices"

type paramKey struct {
	g    *Func
	i    int
	kind siteKind
}

// siteKind is the question a parameter summary answers; each lists paths under the parameter.
type siteKind uint8

const (
	// siteStores: where the callee may store, or hand a `&mut` from the parameter to code that may.
	siteStores siteKind = iota
	// siteMoves: where it may take something out and put it in another place or return it.
	siteMoves
	// siteSelf: where the parts it may store back under the same parameter are read from.
	siteSelf
	// siteOut: where it may put a part of the parameter that carries a borrow into another place or return it.
	siteOut
)

// wholeParam is the answer that assumes anything: a store anywhere under the parameter, through what it holds too.
var wholeParam = []paramStore{{via: true, callee: relayTarget{kind: relayUnknown}}}

// path is a prefix, empty for the whole parameter; beyond lies past a reference the place holds, leaving its fields;
// via marks a store that may go through such a reference, so only it reaches the places the reference points at.
type paramStore struct {
	path   stepPath
	beyond bool
	via    bool
	callee relayTarget
}

func (s paramStore) same(o paramStore) bool {
	return s.beyond == o.beyond && s.via == o.via && s.callee == o.callee && slices.Equal(s.path, o.path)
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
func (c *borrowCheck) paramStoreSites(g *Func, i int) []paramStore {
	return c.paramSites(paramKey{g: g, i: i})
}

// paramMoves reports whether g may put something taken out of parameter i into another place or return it.
func (c *borrowCheck) paramMoves(g *Func, i int) bool {
	return len(c.paramSites(paramKey{g: g, i: i, kind: siteMoves})) > 0
}

func (c *borrowCheck) paramOut(g *Func, i int) bool {
	return len(c.paramSites(paramKey{g: g, i: i, kind: siteOut})) > 0
}

func (c *borrowCheck) paramSelfSources(g *Func, i int) []stepPath {
	var out []stepPath
	for _, s := range c.paramSites(paramKey{g: g, i: i, kind: siteSelf}) {
		out = append(out, s.path)
	}
	return out
}

// Mutually recursive summaries settle together as one strongly connected component (Tarjan), iterated to a fixpoint.
func (c *borrowCheck) paramSites(key paramKey) []paramStore {
	g, i := key.g, key.i
	if g == nil || i >= len(g.Params) {
		return wholeParam
	}
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
	e.sites = c.collectStoreSites(key, e.from)
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
			if s := c.collectStoreSites(k, m.from); !sameSites(s, m.sites) {
				m.sites, changed = s, true
			}
		}
		if !changed {
			break
		}
		if round == 64 {
			for _, k := range members {
				c.storeMemo[k].sites = wholeParam
			}
			break
		}
	}
	for _, k := range c.siteSCC.pop(slot) {
		c.storeMemo[k].from = nil
	}
}
