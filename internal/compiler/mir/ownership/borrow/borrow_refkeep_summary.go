package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// refKeepMemo holds refKeeps by bound body, scc the entries whose call-graph component is still open, and the
// witnessSet answers with the uses they are read from, wscc those of a cycle still being solved.
type refKeepMemo struct {
	sums      map[keptKey]*refKeepEntry
	scc       sccStack[keptKey]
	low       int
	uses      map[sem.EntityID][]witnessUse
	witnesses map[witnessKey]*witnessEntry
	wscc      sccStack[witnessKey]
	wlow      int
}

type refKeepEntry struct {
	s    *refSummary
	g    *midir.Func
	bind fnBind
}

// refKeeps is what e's body may keep of what its argument slots lend, when a callable it cannot rule out keeping one
// (a witness) is handed it; nil without a body. Recursive bodies start from nothing and settle together.
func (c *borrowCheck) refKeeps(e sem.EntityID, bind fnBind) *refSummary {
	g := c.p.ByEnt[e]
	if g == nil {
		return nil
	}
	m := &c.state.refKeeps
	key := keptKey{g: g, bind: bind.key()}
	if en, ok := m.sums[key]; ok {
		if slot, open := m.scc.slot(key); open {
			m.low = min(m.low, slot)
		}
		return en.s
	}
	if m.sums == nil {
		m.sums = map[keptKey]*refKeepEntry{}
	}
	en := &refKeepEntry{s: &refSummary{}, g: g, bind: bind}
	m.sums[key] = en
	slot := m.scc.push(key)
	outer := m.low
	m.low = noCut
	en.s = c.refKeepRun(g, bind)
	low := m.low
	if low < slot {
		m.low = min(outer, low)
		return en.s
	}
	c.settleRefKeeps(slot, low == slot)
	m.low = outer
	return en.s
}

// settleRefKeeps closes the component open from slot up, rerunning its members, with any a rerun opens, until none
// changes when they recurse.
func (c *borrowCheck) settleRefKeeps(slot int, recursive bool) {
	m := &c.state.refKeeps
	for round := 0; recursive; round++ {
		members := m.scc.members(slot)
		changed := false
		for _, k := range slices.Backward(members) {
			en := m.sums[k]
			if s := c.refKeepRun(en.g, en.bind); !s.same(en.s) {
				en.s, changed = s, true
			}
		}
		if !changed && len(m.scc.members(slot)) == len(members) {
			break
		}
		if round == 64 {
			for _, k := range m.scc.members(slot) {
				m.sums[k].s = &refSummary{all: true}
			}
			break
		}
	}
	m.scc.pop(slot)
}

// refKeepRun runs keptBody's refs analysis once per window of 32 argument slots, a closure's captures counted after its
// parameters; the runs are independent since every step moves each slot's bits on its own.
func (c *borrowCheck) refKeepRun(g *midir.Func, bind fnBind) *refSummary {
	slots := len(c.declaredParams(g.Ent))
	if c.p.R.Entity(g.Ent).Kind == sem.EntClosure {
		slots += len(c.hiddenLocals(g.Ent))
	}
	s := &refSummary{}
	for base := 0; base < max(slots, 1); base += contentShift {
		s.addWindow(c.keptBody(g, bind.ents(), &refRun{bind: bind, base: base}), base)
	}
	return s
}
