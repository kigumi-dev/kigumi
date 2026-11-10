package borrow

import (
	"maps"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// callablesIn is what callables value a may hold: fns and closures its facts and type name, the binding of the
// parameter it comes from, unknown for a value from anywhere else.
func (r *taskRun) callablesIn(a midir.LocalID) bindVal {
	if j := r.slot(r.copiedRoot(a)); j >= 0 {
		return r.paramBinding(j)
	}
	var v bindVal
	for _, f := range r.callableFacts(a, 8) {
		switch e := sem.EntityID(f); {
		case f&offBit != 0:
		case f == opaqueFact:
			v.unknown = true
		case r.c.p.R.Entity(e).Kind == sem.EntFn || r.c.p.R.Entity(e).Kind == sem.EntClosure:
			v = v.join(bindVal{ents: []sem.EntityID{e}})
		case r.c.p.R.Entity(e).Kind == sem.EntParam:
			v = v.join(r.paramEntBinding(e))
		default:
			v.unknown = true
		}
	}
	t := r.g.Locals[a].Type
	v = v.join(bindVal{ents: r.c.typeClosures(t)})
	if v.unknown {
		if items := r.c.within(r.g).fnItemsWithin(a, 8); items.known {
			v = bindVal{ents: v.ents, param: v.param}.join(bindVal{ents: items.ents})
		} else if r.c.onlyClosures(t) {
			v.unknown = false
		}
	}
	return v
}

// callableFacts are a's facts without the unknown the body facts add to every callable a call returns, when the
// summaries of that call's callees, for the arguments given, return nothing unknown.
func (r *taskRun) callableFacts(a midir.LocalID, depth int) []uint32 {
	fs := r.flow()
	own := slices.Collect(maps.Keys(fs[a]))
	if !fs[a][opaqueFact] || r.returnsUnknown(r.copiedRoot(a), depth) {
		return own
	}
	return slices.DeleteFunc(own, func(f uint32) bool { return f == opaqueFact })
}

func (r *taskRun) returnsUnknown(x midir.LocalID, depth int) bool {
	defs := r.defs[x]
	if depth == 0 || len(defs) != 1 || defs[0].Op != midir.OpCall && defs[0].Op != midir.OpCallValue {
		return true
	}
	in := defs[0]
	ents := r.c.knownCallees(r.g, r.flow(), *in)
	if ents == nil {
		return true
	}
	for _, e := range ents {
		sum, ok := r.c.summaryOf(e)
		if !ok {
			return true
		}
		arg := func(q sem.EntityID) []uint32 {
			var facts []uint32
			for k, x := range in.Args {
				if slices.Contains(r.c.boundParams([]sem.EntityID{e}, *in, k), q) {
					facts = append(facts, r.callableFacts(x, depth-1)...)
				}
			}
			return facts
		}
		if slices.Contains(r.c.intoCaller(e, sum.result, arg), opaqueFact) {
			return true
		}
	}
	return false
}

func (r *taskRun) copiedRoot(l midir.LocalID) midir.LocalID {
	for range 8 {
		defs := r.defs[l]
		if len(defs) != 1 || len(defs[0].Args) == 0 {
			return l
		}
		switch defs[0].Op {
		case midir.OpCopy, midir.OpMove, midir.OpAlias:
			l = defs[0].Args[0]
		default:
			return l
		}
	}
	return l
}

// onlyClosures reports a type whose every callable part is a closure its type names.
func (c *borrowCheck) onlyClosures(t sem.TypeID) bool {
	return !c.typeHolds(t, func(t sem.TypeID) bool {
		n := c.p.R.Types.Node(t)
		return c.callableNode(t) && !(n.Kind == sem.KClosure && n.Ent != 0)
	})
}
