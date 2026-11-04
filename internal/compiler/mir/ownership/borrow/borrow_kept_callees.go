package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) resolveKeptCallees(ents []sem.EntityID) []sem.EntityID {
	var out []sem.EntityID
	for _, e := range ents {
		if !c.ifaceRequirement(e) {
			out = append(out, e)
		} else if ws := c.implementers(e); len(ws) > 0 {
			out = append(out, ws...)
		} else {
			out = append(out, e)
		}
	}
	return out
}

// keptFlowCallees are the fns call in runs, each with the parameters it takes, and the call's own arguments; no fn
// when a value call's origins are unknown.
func (c *borrowCheck) keptFlowCallees(in midir.Inst) ([]callTarget, []midir.LocalID) {
	args := in.Args
	var ts []callTarget
	switch {
	case in.Op != midir.OpCallValue && in.Op != midir.OpFutureNew:
		ts = []callTarget{{ent: in.Ent, args: args}}
	case in.Ent != 0:
		args = in.Args[1:]
		ts = []callTarget{{ent: in.Ent, args: args}}
	default:
		args = in.Args[1:]
		if d := c.definition(in.Args[0]); d != nil && d.Op == midir.OpFnItem {
			ts = []callTarget{{ent: d.Ent, args: args}}
			break
		}
		origins, ok := c.calleeOrigins(c.f, c.soleDefMap(), in.Args[0])
		if !ok || origins[0].isParam {
			return nil, args
		}
		for _, o := range origins {
			ts = append(ts, callTarget{ent: o.ent, args: o.params(args)})
		}
	}
	var out []callTarget
	for _, t := range ts {
		for _, e := range c.resolveKeptCallees([]sem.EntityID{t.ent}) {
			out = append(out, callTarget{ent: e, args: t.args})
		}
	}
	return out, args
}

func (c *borrowCheck) summarizedValueCallees(in midir.Inst) []callTarget {
	if in.Op != midir.OpCallValue || in.Ent != 0 {
		return nil
	}
	ts, _ := c.keptFlowCallees(in)
	if slices.ContainsFunc(ts, func(t callTarget) bool { return c.unsummarized(t.ent) }) {
		return nil
	}
	return ts
}
