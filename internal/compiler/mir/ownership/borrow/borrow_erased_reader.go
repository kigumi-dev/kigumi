package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// erasedReaders finds the plain `fn` values made from closures sharing a cell local, and every value holding one.
type erasedReaders struct {
	bc *borrowCheck
	p  *ownquery.Input
	f  *midir.Func
	// closures carry cell places as closure values; caps carry them as plain `fn` values, the holders the check follows.
	closures map[midir.LocalID][]capturedPlace
	caps     map[midir.LocalID][]capturedPlace
	// site is where the closure a holder runs became a plain `fn`; found keeps the earliest write seen for each site.
	site  map[midir.LocalID]syntax.NodeID
	found map[syntax.NodeID]erasedWrite
	prior map[reportKey]int
	fates fateMemo
	defs  map[midir.LocalID]*midir.Inst
	win   erasedWindow
}

type erasedWrite struct {
	write syntax.NodeID
	root  midir.LocalID
	how   erasedHow
}

// checkErasedReaders: a plain `fn` cannot share a cell, so the local is not written between the making of a closure reading
// it and the last run of the plain `fn` made from it; sem judges writes made inside closures.
func checkErasedReaders(state *Analyzer, f *midir.Func, prior map[reportKey]int) {
	p := state.Input
	if !slices.ContainsFunc(f.Blocks, func(b midir.Block) bool {
		return slices.ContainsFunc(b.Insts, func(in midir.Inst) bool { return storageClosure(p, in, lentReach) })
	}) {
		return
	}
	e := &erasedReaders{bc: quietBorrowCheck(state, f), p: p, f: f, closures: map[midir.LocalID][]capturedPlace{}, caps: map[midir.LocalID][]capturedPlace{},
		site: map[midir.LocalID]syntax.NodeID{}, found: map[syntax.NodeID]erasedWrite{}, prior: prior,
		fates: fateMemo{results: map[resultKey]paramFate{}, checks: map[*midir.Func]*borrowCheck{}}, defs: mirflow.SoleDefs(f)}
	e.solve()
	if len(e.caps) > 0 {
		c := newReadCheck(p, f)
		c.emit = e.note
		c.dropUse = e.dropRuns
		c.escapeFn = e.escapes
		c.scan(e.caps, e.writes())
		e.leaving()
	}
	e.lentReaders()
	e.flush()
}

func storageClosure(p *ownquery.Input, in midir.Inst, reach capReach) bool {
	return in.Op == midir.OpClosure && p.R.Entity(in.Ent).Kind == sem.EntClosure &&
		slices.ContainsFunc(p.R.Closure(in.Ent).Captures, func(cp sem.Capture) bool { return reach(cp.Mode) })
}

// solve follows the shared value-flow edges (copies, containers, field and payload reads, call results, stores).
func (e *erasedReaders) solve() {
	for changed := true; changed; {
		changed = false
		for bi := range e.f.Blocks {
			for _, in := range e.f.Blocks[bi].Insts {
				if in.Op == midir.OpDrop {
					continue
				}
				if storageClosure(e.p, in, cellReach) && addPlaces(e.closures, in.Dst, e.cellCaptures(in)) {
					changed = true
				}
				dsts, srcs := e.bc.flowEdges(in)
				for _, d := range dsts {
					for _, s := range srcs {
						if e.flow(in, d, s) {
							changed = true
						}
					}
				}
			}
		}
	}
}

// flow moves what src carries into dst: a closure value stays one while dst's type still names a closure type or a
// type parameter and becomes a plain `fn` otherwise; a plain `fn` stays one wherever it goes.
func (e *erasedReaders) flow(in midir.Inst, dst, src midir.LocalID) bool {
	if dst == src || int(dst) >= len(e.f.Locals) || !holdsCallable(e.bc, e.f.Locals[dst].Type) {
		return false
	}
	if in.Op.Yields() && dst == in.Dst && !e.resultMayHold(in, src) {
		return false
	}
	grew := false
	if cps := e.caps[src]; len(cps) > 0 && addPlaces(e.caps, dst, cps) {
		grew = true
		e.siteOf(dst, e.site[src])
	}
	if cps := e.closures[src]; len(cps) > 0 {
		if e.bc.typeHolds(e.f.Locals[dst].Type, func(t sem.TypeID) bool { k := e.p.R.Types.Kind(t); return k == sem.KClosure || k == sem.KParam }) {
			grew = addPlaces(e.closures, dst, cps) || grew
		} else if addPlaces(e.caps, dst, cps) {
			grew = true
			e.siteOf(dst, in.Node)
		}
	}
	return grew
}

func (e *erasedReaders) siteOf(l midir.LocalID, n syntax.NodeID) {
	if _, ok := e.site[l]; !ok && n != 0 {
		e.site[l] = n
	}
}

func (e *erasedReaders) cellCaptures(in midir.Inst) []capturedPlace {
	var out []capturedPlace
	for ci, cp := range e.p.R.Closure(in.Ent).Captures {
		if cp.Mode != sem.CapCell || ci >= len(in.Args) {
			continue
		}
		if r := in.Args[ci]; e.f.Locals[r].Cell && e.f.Locals[r].Ent == cp.Local {
			out = append(out, capturedPlace{root: r})
		}
	}
	return out
}

func addPlaces(m map[midir.LocalID][]capturedPlace, l midir.LocalID, cps []capturedPlace) bool {
	grew := false
	for _, cp := range cps {
		if !hasCapturedPlace(m[l], cp) {
			m[l] = append(m[l], cp)
			grew = true
		}
	}
	return grew
}

// writes are the direct writes of a shared cell: a store into it or into a part of its content, or a `&mut` of either.
func (e *erasedReaders) writes() []ownquery.MoveEvent {
	roots := map[midir.LocalID]bool{}
	for _, cps := range e.caps {
		for _, cp := range cps {
			roots[cp.root] = true
		}
	}
	var out []ownquery.MoveEvent
	for bi := range e.f.Blocks {
		for i, in := range e.f.Blocks[bi].Insts {
			switch {
			case len(in.Args) == 0:
				continue
			case in.Op == midir.OpCellSet, in.Op == midir.OpBorrow && in.Index == 1, in.Op == midir.OpSetField, in.Op == midir.OpSetIndex:
			default:
				continue
			}
			if r := cellRoot(e.defs, in.Args[0]); roots[r] {
				out = append(out, ownquery.MoveEvent{At: ownquery.MovePosition{Bid: midir.BlockID(bi), Idx: i}, Root: r, Node: in.Node})
			}
		}
	}
	return out
}

// cellRoot is the cell local whose content l is, or a part of, else l itself.
func cellRoot(defs map[midir.LocalID]*midir.Inst, l midir.LocalID) midir.LocalID {
	for d := defs[l]; d != nil; d = defs[l] {
		switch d.Op {
		case midir.OpField, midir.OpIndex, midir.OpPayload:
			l = d.Args[0]
			continue
		case midir.OpCellGet:
			return d.Args[0]
		}
		break
	}
	return l
}
