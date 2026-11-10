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
	refKeeps    refKeepMemo
	paramStores map[paramSlot]bool
	ended       map[*midir.Func]*endedFacts
	taskFates   taskFates
	taskCalls   map[*midir.Func]srcSet
	// taskStarts is whether a lowered body names a declaration that starts a Task (0 not known, 1 no, 2 yes), checked
	// over taskStartsSeen bodies.
	taskStarts     uint8
	taskStartsSeen int
	taskStarters   map[sem.EntityID]bool
}

func New(p *ownquery.Input, lookup BodyLookup) *Analyzer {
	return &Analyzer{Input: p, lookup: lookup}
}

// Without a lookup, an unseen body cannot prove which argument supplies the result.
func (c *borrowCheck) calleeBody(ent sem.EntityID) (*midir.Func, bool) {
	lookup := c.state.lookup
	if lookup == nil {
		return nil, false
	}
	return lookup(ent)
}

func (p *Analyzer) unownedRefs(f *midir.Func) []bool { return p.unowned.Of(p.R, f) }

func (p *Analyzer) HasFlowSummary(f *midir.Func) bool {
	_, ok := p.offloads.body[f]
	return ok
}
