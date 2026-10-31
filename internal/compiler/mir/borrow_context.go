package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type bodyLookup func(sem.EntityID) (*midir.Func, bool)

type borrowAnalysis struct {
	ownershipFacts
	lookup      bodyLookup
	offloads    offloadMemo
	paramStores map[paramSlot]bool
}

func newBorrowAnalysis(p *Program, lookup bodyLookup) *borrowAnalysis {
	return &borrowAnalysis{ownershipFacts: ownershipFacts{Program: p}, lookup: lookup}
}

func (c *borrowCheck) analysis() *borrowAnalysis {
	if c.state == nil {
		c.state = newBorrowAnalysis(c.p, nil)
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
