package mir

import (
	"math"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// sinkMemo keeps sharedSinks by body and binding. A body on the stack is open at its depth; part holds the answers of
// bodies whose cycle is still being solved, with the round they were computed in and the shallowest open body they asked.
type sinkMemo struct {
	done       map[keptKey][]sem.EntityID
	open       map[keptKey]int
	part       map[keptKey]sinkPart
	round, low int
	grew       bool
}

type sinkPart struct {
	s          []sem.EntityID
	round, low int
}

func (c *borrowCheck) sinkState() *sinkMemo {
	m := c.offloads()
	if m.shared == nil {
		m.shared = &sinkMemo{done: map[keptKey][]sem.EntityID{}, open: map[keptKey]int{}, part: map[keptKey]sinkPart{},
			low: math.MaxInt}
	}
	return m.shared
}

// sharedSinks are e's declared parameters its body may store into shared storage, bind's parameters calling those
// callables: none for a body nothing reaches, every one for a body not here; a cycle repeats until no body in it grows.
func (c *borrowCheck) sharedSinks(e sem.EntityID, bind map[int][]sem.EntityID) []sem.EntityID {
	g := c.p.ByEnt[e]
	if g == nil && c.p.hasLowerableBody(e) {
		return nil
	}
	if g == nil {
		return c.declaredParams(e)
	}
	key, m := keptKey{g: g, bind: bindKey(bind)}, c.sinkState()
	if s, ok := m.done[key]; ok {
		return s
	}
	if d, ok := m.open[key]; ok {
		m.low = min(m.low, d)
		return m.part[key].s
	}
	if p, ok := m.part[key]; ok && p.round == m.round {
		m.low = min(m.low, p.low)
		return p.s
	}
	depth, outer, outerGrew, prev := len(m.open), m.low, m.grew, m.part[key].s
	m.open[key] = depth
	grew := false
	for {
		m.low, m.grew = math.MaxInt, false
		m.part[key] = sinkPart{s: prev, round: m.round, low: depth}
		s := c.bodySinks(g, bind, prev)
		grew = grew || len(s) > len(prev)
		prev = s
		if m.low != depth || len(s) == len(m.part[key].s) && !m.grew {
			break
		}
		m.round++
	}
	low := m.low
	delete(m.open, key)
	delete(m.part, key)
	if low < depth {
		m.part[key] = sinkPart{s: prev, round: m.round, low: low}
		m.low, m.grew = min(outer, low), outerGrew || m.grew || grew
		return prev
	}
	m.done[key] = prev
	for k, p := range m.part {
		if p.low >= depth {
			if p.round == m.round {
				m.done[k] = p.s
			}
			delete(m.part, k)
		}
	}
	m.low, m.grew = outer, outerGrew
	return prev
}

// bodySinks adds to from g's declared parameters some call of g may store into shared storage.
func (c *borrowCheck) bodySinks(g *midir.Func, bind map[int][]sem.EntityID, from []sem.EntityID) []sem.EntityID {
	declared := c.declaredParams(g.Ent)
	fs := c.bodyFacts(g)
	if fs == nil {
		return declared
	}
	x := &sinkCtx{g: g, fs: fs, bind: bind}
	out := slices.Clone(from)
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			for _, st := range c.sharedStoreArgs(x, in) {
				for f := range fs[in.Args[st.arg]] {
					if q := sem.EntityID(f); f != opaqueFact && slices.Contains(declared, q) && !slices.Contains(out, q) {
						out = append(out, q)
					}
				}
			}
		}
	}
	return out
}

func (c *borrowCheck) ifaceRequirement(e sem.EntityID) bool {
	if m := c.methodOf(e); m != 0 {
		e = m
	}
	return c.p.R.Entity(e).Kind == sem.EntFn && c.p.R.Fn(e).Owner != 0 && c.p.R.Entity(c.p.R.Fn(e).Owner).Kind == sem.EntInterface
}

// implementers are the methods the conformances sem settled before sealing its witness table name for requirement req;
// none when there is no such record, which leaves the call unknown.
func (c *borrowCheck) implementers(req sem.EntityID) []sem.EntityID {
	ws, _ := c.p.R.SealedWitnesses(req)
	return ws
}
