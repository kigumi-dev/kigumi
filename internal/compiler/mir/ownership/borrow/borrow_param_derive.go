package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// beyond marks a walk that read a reference stored in the place on every way, via on some way.
type derivation struct {
	path   ownquery.Path
	cut    bool
	beyond bool
	via    bool
}

// Two ways to one local keep their common prefix.
func (c *borrowCheck) paramDerivations(g *midir.Func, p midir.LocalID) map[midir.LocalID]derivation {
	from := map[midir.LocalID]derivation{p: {}}
	defs := mirflow.SoleDefs(g)
	def := func(l midir.LocalID) *midir.Inst { return defs[l] }
	for changed := true; changed; {
		changed = false
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if !derives[in.Op] && !c.opaqueResult(&in) {
					continue
				}
				k := slices.IndexFunc(in.Args, func(a midir.LocalID) bool { _, ok := from[a]; return ok })
				if k < 0 {
					continue
				}
				d := c.deriveStep(g, &in, from[in.Args[k]], k == 0, def)
				if old, ok := from[in.Dst]; !ok {
					from[in.Dst], changed = d, true
				} else if m := joinDerivation(old, d); !sameDerivation(m, old) {
					from[in.Dst], changed = m, true
				}
			}
		}
	}
	return from
}

// A loop element lies at some element of its collection, and another opaque result anywhere below its argument.
func (c *borrowCheck) deriveStep(g *midir.Func, in *midir.Inst, d derivation, base bool, def func(midir.LocalID) *midir.Inst) derivation {
	out := derivation{path: d.path, cut: d.cut || !base, beyond: d.beyond, via: d.via}
	step, read := ownquery.PathStep(in, def)
	if in.Op == midir.OpUnbox {
		out.cut, read = true, true
	}
	held := read && c.p.R.Types.Kind(g.Locals[in.Dst].Type) == sem.KRef
	out.beyond, out.via = out.beyond || held, out.via || held
	switch {
	case iterElement(in):
		step, read = ownquery.AnyElem, true
	case !derives[in.Op]:
		out.cut = true
	}
	if read && !out.cut {
		out.cut = len(out.path) >= ownquery.MaxPathLen
		out.path = out.path.With(step)
	}
	return out
}

// opaqueResult reports a call whose body the summaries do not read, so its result may be or point into an argument.
func (c *borrowCheck) opaqueResult(in *midir.Inst) bool {
	switch in.Op {
	case midir.OpBuiltin, midir.OpCallValue:
		return true
	case midir.OpCall:
		return c.p.ByEnt[in.Ent] == nil
	}
	return false
}

func joinDerivation(a, b derivation) derivation {
	p := a.path.CommonPrefix(b.path)
	return derivation{path: p, cut: a.cut || b.cut || len(p) < len(a.path) || len(p) < len(b.path), beyond: a.beyond && b.beyond, via: a.via || b.via}
}

func sameDerivation(a, b derivation) bool {
	return slices.Equal(a.path, b.path) && a.cut == b.cut && a.beyond == b.beyond && a.via == b.via
}

// carries reports whether a value of type t may hold what the flows track: something callable, a Future or Task, a
// borrow, a resource whose drop runs, or a type parameter.
func (c *borrowCheck) carries(t sem.TypeID) bool {
	tt := c.p.R.Types
	return c.typeRuns(t, runsDrop) || c.typeHolds(t, func(t sem.TypeID) bool {
		switch tt.Kind(t) {
		case sem.KFn, sem.KClosure, sem.KIface, sem.KRef, sem.KParam, sem.KVar:
			return true
		}
		return tt.Kind(t) == sem.KNamed && tt.Node(t).Ent == tt.FutureEnt() || c.p.R.IsTaskType(t)
	})
}
