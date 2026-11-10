package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type fateKey struct {
	g    *midir.Func
	bind string
}

// taskFates memoizes taskFate per body and binding. A recursion settles by iterating from the optimistic fate; a
// summary that read one still settling above it stays open, reused until a fate it may read changes and then resumed.
type taskFates struct {
	done map[fateKey]*taskFate
	open map[fateKey]*openFate
	// ids names the summaries being computed, by depth; reach is the shallowest depth a summary read an open one at.
	ids   []int
	next  int
	reach int
	stamp int
	seq   int
}

// openFate is a summary on the stack (depth >= 0) or one that read the open summary at depth reach, valid while that
// one is still there and no fate changed since stamp.
type openFate struct {
	f       *taskFate
	depth   int
	reach   int
	reachID int
	stamp   int
	seq     int
}

func (c *borrowCheck) taskFateOf(e sem.EntityID, bind taskBind) *taskFate {
	if m := c.methodOf(e); m != 0 {
		e = m
	}
	g := c.p.ByEnt[e]
	if g == nil {
		return c.signatureFate(e)
	}
	m := &c.state.taskFates
	if m.done == nil {
		m.done, m.open = map[fateKey]*taskFate{}, map[fateKey]*openFate{}
	}
	key := fateKey{g, bind.key()}
	if f, ok := m.done[key]; ok {
		return f
	}
	if o := m.open[key]; o != nil && (o.depth >= 0 || o.stamp == m.stamp && o.reach < len(m.ids) && m.ids[o.reach] == o.reachID) {
		m.reach = min(m.reach, max(o.depth, o.reach))
		return o.f
	}
	return c.settleFate(g, bind, key)
}

func (c *borrowCheck) settleFate(g *midir.Func, bind taskBind, key fateKey) *taskFate {
	m := &c.state.taskFates
	d := len(m.ids)
	m.next++
	m.ids = append(m.ids, m.next)
	start := &taskFate{joins: allBits(64), nslot: len(c.declaredParams(g.Ent)) + len(c.hiddenLocals(g.Ent))}
	if stale := m.open[key]; stale != nil {
		start = stale.f
	}
	o := &openFate{f: start, depth: d}
	m.open[key] = o
	outer, since := m.reach, m.seq
	var f *taskFate
	for {
		m.reach, since = d+1, m.seq
		f = c.summarizeTasks(g, bind)
		if m.reach > d || f.equal(o.f) {
			break
		}
		o.f = f
		m.stamp++
	}
	m.ids = m.ids[:d]
	if m.reach >= d {
		delete(m.open, key)
		m.done[key] = f
		for k, p := range m.open {
			if p.depth < 0 && p.stamp == m.stamp && p.seq > since {
				m.done[k] = p.f
				delete(m.open, k)
			}
		}
	} else {
		m.seq++
		*o = openFate{f: f, depth: -1, reach: m.reach, reachID: m.ids[m.reach], stamp: m.stamp, seq: m.seq}
	}
	m.reach = min(outer, m.reach)
	return f
}
