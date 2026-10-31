package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// erasedWindow finds what runs between the making of a closure and the call it is lent to as a plain `fn`.
type erasedWindow struct {
	made    map[sem.EntityID][]ownquery.MovePosition
	uses    map[midir.LocalID][]ownquery.MovePosition
	preds   [][]midir.BlockID
	chained map[midir.LocalID]bool
}

// earlierWriter is an instruction run after r's closure was made and before the call at writes r's place: a store, a
// `&mut`, a call writing through a write-back cell, or a call that may run a closure writing it.
func (e *erasedReaders) earlierWriter(at ownquery.MovePosition, r lentRead) (syntax.NodeID, bool) {
	e.windowInit()
	for _, from := range e.win.made[r.cl] {
		win := e.window(from, at)
		for bi := range e.f.Blocks {
			for i, w := range e.f.Blocks[bi].Insts {
				p := ownquery.MovePosition{Bid: midir.BlockID(bi), Idx: i}
				if !win[p] || w.Op == midir.OpClosure || e.roundTrip(w, r.o.owner, win) {
					continue
				}
				if u, ok := cellAccess(e.f, e.defs, e.win.chained, r.o.owner, w); ok && u.writes && ownquery.FieldPathsOverlap(u.path, r.o.path) {
					return w.Node, true
				}
				if slices.ContainsFunc(e.callWrites(w, r.o.owner), func(q ownquery.Path) bool { return ownquery.FieldPathsOverlap(q, r.o.path) }) {
					return w.Node, true
				}
				e.bc.curBlock, e.bc.curIdx = p.Bid, p.Idx
				if hit, _ := e.bc.runHitVia(w, r.o, nil, storageReach); hit {
					return w.Node, true
				}
			}
		}
	}
	return 0, false
}

func (e *erasedReaders) windowInit() {
	if e.win.made != nil {
		return
	}
	e.win = erasedWindow{made: map[sem.EntityID][]ownquery.MovePosition{}, uses: map[midir.LocalID][]ownquery.MovePosition{}, preds: make([][]midir.BlockID, len(e.f.Blocks)), chained: chainedTemps(e.f)}
	for bi, blk := range e.f.Blocks {
		for i, in := range blk.Insts {
			p := ownquery.MovePosition{Bid: midir.BlockID(bi), Idx: i}
			if in.Op == midir.OpClosure {
				e.win.made[in.Ent] = append(e.win.made[in.Ent], p)
			}
			for _, a := range in.Args {
				e.win.uses[a] = append(e.win.uses[a], p)
			}
		}
		for _, a := range blk.Term.Args {
			e.win.uses[a] = append(e.win.uses[a], ownquery.MovePosition{Bid: midir.BlockID(bi), Idx: -1})
		}
		for _, t := range blk.Term.Targets {
			e.win.preds[t] = append(e.win.preds[t], midir.BlockID(bi))
		}
	}
}

// window is every position on a path from `from` to `to` that passes through neither of them again.
func (e *erasedReaders) window(from, to ownquery.MovePosition) map[ownquery.MovePosition]bool {
	stop := func(p ownquery.MovePosition) bool { return p == from || p == to }
	fwd, bwd := e.walk(from, stop, 1), e.walk(to, stop, -1)
	for p := range fwd {
		if !bwd[p] {
			delete(fwd, p)
		}
	}
	return fwd
}

func (e *erasedReaders) walk(from ownquery.MovePosition, stop func(ownquery.MovePosition) bool, step int) map[ownquery.MovePosition]bool {
	seen, entered := map[ownquery.MovePosition]bool{}, map[midir.BlockID]bool{}
	var run func(b midir.BlockID, i int)
	run = func(b midir.BlockID, i int) {
		for ; i >= 0 && i < len(e.f.Blocks[b].Insts); i += step {
			p := ownquery.MovePosition{Bid: b, Idx: i}
			if stop(p) || seen[p] {
				return
			}
			seen[p] = true
		}
		next := e.f.Blocks[b].Term.Targets
		if step < 0 {
			next = e.win.preds[b]
		}
		for _, t := range next {
			if !entered[t] {
				entered[t] = true
				start := 0
				if step < 0 {
					start = len(e.f.Blocks[t].Insts) - 1
				}
				run(t, start)
			}
		}
	}
	run(from.Bid, from.Idx+step)
	return seen
}
