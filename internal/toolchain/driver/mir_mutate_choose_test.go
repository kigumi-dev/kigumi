package driver_test

import (
	"fmt"
	"math/rand"
	"strconv"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// pick is one choice an edit made. A recorded pick holds the raw id the edit
// chose; a pick parsed from a recipe holds the name that finds it again.
type pick struct {
	kind  byte
	n     int
	ekind sem.EntityKind
	text  string
	nth   int
	oob   bool
	// recorded marks a choice made in this process, whose n is an id and not a name.
	recorded bool
}

// chooser draws an edit's choices from a generator and records them, or replays recorded ones.
type chooser struct {
	rnd    *rand.Rand
	script []pick
	next   int
	rec    []pick
}

func (c *chooser) replaying() bool { return c.rnd == nil }

func (c *chooser) draw(n int) int { return c.rnd.Intn(n) }

func (c *chooser) note(p pick) {
	p.recorded = true
	c.rec = append(c.rec, p)
}

func (c *chooser) take(kind byte) pick {
	if c.next >= len(c.script) {
		panic(staleRecipe("the edit asks for more choices than the recipe holds"))
	}
	p := c.script[c.next]
	c.next++
	if p.kind != kind {
		panic(staleRecipe(fmt.Sprintf("the edit asks for a %c choice where the recipe holds a %c", kind, p.kind)))
	}
	return p
}

func (c *chooser) Intn(n int) int {
	if c.replaying() {
		p := c.take('i')
		if p.n < 0 || p.n >= n {
			panic(staleRecipe(fmt.Sprintf("the recipe chose %d of %d", p.n, n)))
		}
		return p.n
	}
	v := c.rnd.Intn(n)
	c.note(pick{kind: 'i', n: v})
	return v
}

type staleRecipe string

func (s staleRecipe) Error() string { return "stale recipe: " + string(s) }

func (s *site) local() mir.LocalID {
	if s.r.replaying() {
		p := s.r.take('l')
		if p.oob {
			return mir.LocalID(len(s.f.Locals) + p.nth)
		}
		seen := 0
		for i, l := range s.f.Locals {
			if l.Name == p.text {
				if seen == p.nth {
					return mir.LocalID(i)
				}
				seen++
			}
		}
		panic(staleRecipe(fmt.Sprintf("%s has no local %s#%d", s.f.Name, strconv.Quote(p.text), p.nth)))
	}
	var l mir.LocalID
	if len(s.f.Locals) == 0 || s.r.draw(24) == 0 {
		l = mir.LocalID(len(s.f.Locals) + s.r.draw(2))
	} else {
		l = mir.LocalID(s.r.draw(len(s.f.Locals)))
	}
	s.r.note(pick{kind: 'l', n: int(l)})
	return l
}

func (s *site) typ() sem.TypeID {
	if s.r.replaying() {
		p := s.r.take('t')
		return findType(s.p, p)
	}
	var t sem.TypeID
	if s.r.draw(12) == 0 {
		t = sem.TypeID(s.r.draw(s.p.R.Types.Len()))
	} else {
		t = s.m.types[s.r.draw(len(s.m.types))]
	}
	s.r.note(pick{kind: 't', n: int(t)})
	return t
}

func (s *site) ent(k sem.EntityKind) (sem.EntityID, bool) {
	if s.r.replaying() {
		p := s.r.take('e')
		return findEntity(s.p, p), true
	}
	pool := s.m.ents[k]
	if len(pool) == 0 {
		return 0, false
	}
	e := pool[s.r.draw(len(pool))]
	s.r.note(pick{kind: 'e', n: int(e), ekind: k})
	return e, true
}

func (s *site) str() string {
	if s.r.replaying() {
		return s.r.take('s').text
	}
	v := s.m.strs[s.r.draw(len(s.m.strs))]
	s.r.note(pick{kind: 's', text: v})
	return v
}

func (s *site) prim() opdef.PrimitiveId {
	if s.r.replaying() {
		id, _ := opdef.StdOpIdOf(s.r.take('p').text)
		return opdef.PrimitiveId(id) + 1
	}
	v := s.m.prims[s.r.draw(len(s.m.prims))]
	s.r.note(pick{kind: 'p', n: int(v)})
	return v
}

func (s *site) prov() opdef.ProviderOpId {
	if s.r.replaying() {
		id, _ := opdef.StdOpIdOf(s.r.take('v').text)
		return opdef.ProviderOpId(id) + 1
	}
	v := s.m.provs[s.r.draw(len(s.m.provs))]
	s.r.note(pick{kind: 'v', n: int(v)})
	return v
}
