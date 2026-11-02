package borrow

import (
	"sort"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
)

type endEvent struct {
	block midir.BlockID
	idx   int32
	ends  bool
}

// endedFacts answers endedAt for every root of one function from one index of end events, walking back from a
// query over blocks with no event for the root; no answer is kept per root and block, as that grows quadratically.
type endedFacts struct {
	f       *midir.Func
	first   []int32
	events  []endEvent
	flows   []bool
	preds   [][]midir.BlockID
	reached []bool
	mark    []uint32
	epoch   uint32
	queue   []midir.BlockID
}

// endedOf is never invalidated: a body's MIR no longer changes once its borrow checks run.
func (p *Analyzer) endedOf(f *midir.Func) *endedFacts {
	if e, ok := p.ended[f]; ok {
		return e
	}
	if p.ended == nil {
		p.ended = map[*midir.Func]*endedFacts{}
	}
	e := newEndedFacts(f)
	p.ended[f] = e
	return e
}

func newEndedFacts(f *midir.Func) *endedFacts {
	e := &endedFacts{f: f, first: make([]int32, len(f.Locals)+1), flows: make([]bool, len(f.Locals)), preds: mirflow.Preds(f), reached: make([]bool, len(f.Blocks))}
	for _, b := range mirflow.ReversePostorder(f) {
		e.reached[b] = true
	}
	eachEndEvent(f, func(l midir.LocalID, _ endEvent) { e.first[l+1]++ })
	for l := range f.Locals {
		e.first[l+1] += e.first[l]
	}
	e.events = make([]endEvent, e.first[len(f.Locals)])
	next := append([]int32(nil), e.first[:len(f.Locals)]...)
	eachEndEvent(f, func(l midir.LocalID, ev endEvent) {
		e.events[next[l]] = ev
		next[l]++
	})
	for l := range f.Locals {
		e.flows[l] = e.endLeavesBlock(midir.LocalID(l))
	}
	return e
}

func eachEndEvent(f *midir.Func, fn func(midir.LocalID, endEvent)) {
	for b := range f.Blocks {
		for i := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[i]
			ends := (in.Op == midir.OpDrop || in.Op == midir.OpMove) && len(in.Args) > 0
			if ends {
				fn(in.Args[0], endEvent{block: midir.BlockID(b), idx: int32(i), ends: true})
			}
			if in.Op.Yields() && !(ends && in.Args[0] == in.Dst) {
				fn(in.Dst, endEvent{block: midir.BlockID(b), idx: int32(i)})
			}
		}
	}
}

// endLeavesBlock is whether some reached block with a successor last ends root; without one no entry is ended.
func (e *endedFacts) endLeavesBlock(root midir.LocalID) bool {
	evs := e.events[e.first[root]:e.first[root+1]]
	for i, ev := range evs {
		last := i+1 == len(evs) || evs[i+1].block != ev.block
		if last && ev.ends && e.reached[ev.block] && len(e.f.Blocks[ev.block].Term.Targets) > 0 {
			return true
		}
	}
	return false
}

func (e *endedFacts) before(root midir.LocalID, block midir.BlockID, idx int) bool {
	if ends, ok := e.lastEvent(root, block, idx); ok {
		return ends
	}
	return e.flows[root] && e.reached[block] && e.entry(root, block)
}

func (e *endedFacts) lastEvent(root midir.LocalID, block midir.BlockID, idx int) (ends, ok bool) {
	evs := e.events[e.first[root]:e.first[root+1]]
	i := sort.Search(len(evs), func(i int) bool {
		return evs[i].block > block || evs[i].block == block && int(evs[i].idx) >= idx
	})
	if i == 0 || evs[i-1].block != block {
		return false, false
	}
	return evs[i-1].ends, true
}

func (e *endedFacts) entry(root midir.LocalID, block midir.BlockID) bool {
	if e.mark == nil {
		e.mark = make([]uint32, len(e.f.Blocks))
	}
	e.epoch++
	e.mark[block] = e.epoch
	e.queue = append(e.queue[:0], block)
	for i := 0; i < len(e.queue); i++ {
		for _, p := range e.preds[e.queue[i]] {
			if e.exitEnds(root, p) {
				return true
			}
		}
	}
	return false
}

func (e *endedFacts) exitEnds(root midir.LocalID, p midir.BlockID) bool {
	if !e.reached[p] {
		return false
	}
	if ends, ok := e.lastEvent(root, p, len(e.f.Blocks[p].Insts)); ok {
		return ends
	}
	if e.mark[p] != e.epoch {
		e.mark[p] = e.epoch
		e.queue = append(e.queue, p)
	}
	return false
}
