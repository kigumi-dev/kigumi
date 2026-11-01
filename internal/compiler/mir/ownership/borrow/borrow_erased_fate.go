package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
)

type resultKey struct {
	g   *midir.Func
	idx int
}

// paramFate is where a callee may put what one parameter is handed: into its result, into a place its caller owns
// through another parameter, or somewhere no flow follows.
type paramFate struct {
	result, stores, escapes bool
}

func (a paramFate) join(b paramFate) paramFate {
	return paramFate{result: a.result || b.result, stores: a.stores || b.stores, escapes: a.escapes || b.escapes}
}

// fateMemo holds the settled summaries and, while one is computed, the open ones it reached; recursion reads an open
// summary's current value, starting from nothing, and the open set is iterated until no summary grows.
type fateMemo struct {
	results map[resultKey]paramFate
	open    []resultKey
	depth   int
	checks  map[*midir.Func]*borrowCheck
}

func (e *erasedReaders) fate(g *midir.Func, idx int) paramFate {
	key := resultKey{g, idx}
	m := &e.fates
	if v, ok := m.results[key]; ok {
		return v
	}
	m.results[key] = paramFate{}
	m.open = append(m.open, key)
	m.depth++
	m.results[key] = m.results[key].join(e.fateOnce(g, idx))
	m.depth--
	if m.depth > 0 {
		return m.results[key]
	}
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(m.open); i++ {
			k := m.open[i]
			m.depth++
			v := m.results[k].join(e.fateOnce(k.g, k.idx))
			m.depth--
			if v != m.results[k] {
				m.results[k], changed = v, true
			}
		}
	}
	m.open = nil
	return m.results[key]
}

// fateOnce follows g's parameter idx through g's body once, reading the callees' summaries as they stand.
func (e *erasedReaders) fateOnce(g *midir.Func, idx int) paramFate {
	bc := e.fates.checks[g]
	if bc == nil {
		bc = quietBorrowCheck(e.bc.state, g)
		e.fates.checks[g] = bc
	}
	reach := map[midir.LocalID]bool{g.Params[idx]: true}
	for changed := true; changed; {
		changed = false
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				dsts, srcs := bc.flowEdges(in)
				if in.Op == midir.OpDrop || !slices.ContainsFunc(srcs, func(s midir.LocalID) bool { return reach[s] }) {
					continue
				}
				for _, d := range dsts {
					if int(d) < len(g.Locals) && !reach[d] && holdsCallable(bc, g.Locals[d].Type) {
						reach[d], changed = true, true
					}
				}
			}
		}
	}
	var v paramFate
	for l := range reach {
		v.stores = v.stores || l != g.Params[idx] && callerParam(e.p, g, l)
	}
	defs, held := mirflow.SoleDefs(g), func(l midir.LocalID) bool { return reach[l] }
	for _, blk := range g.Blocks {
		v.result = v.result || blk.Term.Op == midir.TermReturn && slices.ContainsFunc(blk.Term.Args, held)
		for _, in := range blk.Insts {
			v.stores = v.stores || in.Op != midir.OpDrop && slices.ContainsFunc(e.callerStores(g, defs, in, held), func(a midir.LocalID) bool { return a != g.Params[idx] || !callerParam(e.p, g, a) })
			for i, a := range in.Args {
				v.escapes = v.escapes || reach[a] && in.Op != midir.OpDrop && e.escapesAt(g, in, i)
			}
		}
	}
	return v
}
