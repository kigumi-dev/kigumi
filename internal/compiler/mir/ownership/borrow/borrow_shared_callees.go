package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// sinkTarget is a fn or closure a call may run and the declared parameters each operand k of the call binds.
type sinkTarget struct {
	ent  sem.EntityID
	bind func(k int) []sem.EntityID
}

// sinkCallees are what call in may run, false when x cannot name them all: an interface requirement's implementers, on a
// view or through a bound; a closure by its type; else the origins calleeOrigins names, a parameter by its binding.
func (c *borrowCheck) sinkCallees(x *sinkCtx, in midir.Inst) ([]sinkTarget, bool) {
	direct := func(e sem.EntityID) sinkTarget {
		return sinkTarget{ent: e, bind: func(k int) []sem.EntityID { return c.boundParams([]sem.EntityID{e}, in, k) }}
	}
	switch {
	case in.Ent != 0 && c.ifaceRequirement(in.Ent):
		impls := c.implementers(in.Ent)
		return mapTargets(impls, direct), len(impls) > 0
	case in.Op == midir.OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn:
		return []sinkTarget{direct(in.Ent)}, true
	case in.Op == midir.OpCall:
		return nil, false
	case c.calleeClosure(x.g, in) != 0:
		return []sinkTarget{direct(c.calleeClosure(x.g, in))}, true
	}
	if x.defs == nil {
		x.defs = mirflow.SoleDefs(x.g)
	}
	origins, ok := c.calleeOrigins(x.g, x.defs, in.Args[0])
	if !ok {
		return nil, false
	}
	var out []sinkTarget
	for _, o := range origins {
		if o.isParam {
			b, bound := x.bind[slices.Index(c.declaredParams(x.g.Ent), x.g.Locals[x.g.Params[o.param]].Ent)]
			if !bound {
				return nil, false
			}
			out = append(out, mapTargets(b, direct)...)
			continue
		}
		impls := []sem.EntityID{o.ent}
		if c.ifaceRequirement(o.ent) {
			if impls = c.implementers(o.ent); len(impls) == 0 {
				return nil, false
			}
		}
		for _, e := range impls {
			out = append(out, c.originTarget(c.originOf(e, o.layers...)))
		}
	}
	return out, true
}

// originTarget binds a value call's argument k-1 to the declared parameter of o's fn it lands on.
func (c *borrowCheck) originTarget(o calleeOrigin) sinkTarget {
	return sinkTarget{ent: o.ent, bind: func(k int) []sem.EntityID {
		if k == 0 {
			return nil
		}
		if i, ok := c.argDeclared(o, k-1); ok {
			return []sem.EntityID{c.declaredParams(o.ent)[i]}
		}
		return nil
	}}
}

func mapTargets(ents []sem.EntityID, f func(sem.EntityID) sinkTarget) []sinkTarget {
	var out []sinkTarget
	for _, e := range ents {
		out = append(out, f(e))
	}
	return out
}
