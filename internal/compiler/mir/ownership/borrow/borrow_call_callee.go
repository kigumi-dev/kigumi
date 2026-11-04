package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// callTarget is a fn a call runs and the arguments its declared parameters take; kept, set when a function value reaches
// the fn, narrows its signature's flows to what its body stores, as the erasure proofs of such values do.
type callTarget struct {
	ent  sem.EntityID
	args []midir.LocalID
	kept *keptSummary
}

// stores reports whether the fn may put argument j, or what it lends, into the place of argument i.
func (t callTarget) stores(i, j int) bool {
	return t.kept == nil || t.kept.storesInto(i, j) || t.kept.movesInto(i, j)
}

// flowCallees are the fns call in may run. A value call runs the fns calleeOrigins names, each with the parameters its
// value lays out around the call's arguments; none when the origins are unknown.
func (c *borrowCheck) flowCallees(in midir.Inst) []callTarget {
	switch in.Op {
	case midir.OpCall:
		if in.Ent != 0 && c.ifaceRequirement(in.Ent) {
			return c.requirementTargets(in.Ent, in.Args)
		}
		return c.fnTarget(in.Ent, in.Args, false)
	case midir.OpCallValue, midir.OpFutureNew:
	default:
		return nil
	}
	if len(in.Args) == 0 {
		return nil
	}
	callee, args := in.Args[0], in.Args[1:]
	if in.Ent != 0 {
		if c.ifaceRequirement(in.Ent) {
			return c.requirementTargets(in.Ent, args)
		}
		return c.fnTarget(in.Ent, args, false)
	}
	if d := c.definition(callee); d != nil && d.Op == midir.OpFnItem {
		return c.fnTarget(d.Ent, args, false)
	}
	origins, ok := c.calleeOrigins(c.f, c.soleDefMap(), callee)
	if !ok {
		return nil
	}
	var out []callTarget
	for _, o := range origins {
		if !o.isParam {
			out = append(out, c.fnTarget(o.ent, o.params(args), true)...)
		}
	}
	return out
}

// requirementTargets are the implementations a call through a requirement may run: a bodyless requirement carries no
// store in its signature, so each implementation is checked by what its own body stores. The requirement alone when none
// is sealed, which keeps the call opaque.
func (c *borrowCheck) requirementTargets(req sem.EntityID, args []midir.LocalID) []callTarget {
	out := make([]callTarget, 0, 1)
	for _, w := range c.implementers(req) {
		out = append(out, callTarget{ent: w, args: args, kept: c.keptStoreSummary(w, nil)})
	}
	if len(out) == 0 {
		return []callTarget{{ent: req, args: args}}
	}
	return out
}

func (c *borrowCheck) fnTarget(e sem.EntityID, args []midir.LocalID, value bool) []callTarget {
	if e == 0 || c.p.R.Entity(e).Kind != sem.EntFn {
		return nil
	}
	t := callTarget{ent: e, args: args}
	if value {
		t.kept = c.keptStoreSummary(e, c.siteBind(e, args))
	}
	return []callTarget{t}
}
