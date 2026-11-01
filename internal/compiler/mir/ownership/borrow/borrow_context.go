package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

type BodyLookup func(sem.EntityID) (*midir.Func, bool)

type Analyzer struct {
	*ownquery.Input
	unowned     ownquery.LentMemo
	lookup      BodyLookup
	offloads    offloadMemo
	paramStores map[paramSlot]bool
}

func New(p *ownquery.Input, lookup BodyLookup) *Analyzer {
	return &Analyzer{Input: p, lookup: lookup}
}

func (c *borrowCheck) analysis() *Analyzer {
	if c.state == nil {
		c.state = New(c.p, nil)
	}
	return c.state
}

// Without a lookup, an unseen body cannot prove which argument supplies the result.
func (c *borrowCheck) calleeBody(ent sem.EntityID) (*midir.Func, bool) {
	lookup := c.analysis().lookup
	if lookup == nil {
		return nil, false
	}
	return lookup(ent)
}

func (p *Analyzer) unownedRefs(f *midir.Func) []bool { return p.unowned.Of(p.R, f) }
