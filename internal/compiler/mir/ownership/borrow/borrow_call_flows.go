package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// callFlows lists, for each `&mut` / `mut self` argument or by-value `&mut` holder whose place names a lifetime
// or a type parameter, the other arguments whose declared types share a parameter with it, for every fn call in runs.
func (c *borrowCheck) callFlows(in midir.Inst) []storeFlow {
	var out []storeFlow
	for _, t := range c.flowCallees(in) {
		out = append(out, c.targetFlows(t)...)
	}
	return out
}

func (c *borrowCheck) targetFlows(ct callTarget) []storeFlow {
	e, args := ct.ent, ct.args
	info := c.p.R.Fn(e)
	if info.Recv != sem.RecvMut && !slices.ContainsFunc(info.Params, func(p sem.EntityID) bool {
		return c.p.R.Types.RefMut(c.p.R.Entity(p).Type)
	}) && !slices.ContainsFunc(args, func(a midir.LocalID) bool { return c.holdsMutPlace(a) || c.capturesHolder(a) }) {
		return nil
	}
	slots := c.callSlots(info, args, 0)
	c.linkSlots(info, slots)
	var out []storeFlow
	for i, t := range slots {
		if !t.target && !t.holder {
			continue
		}
		kind := c.targetKindOf(t.arg)
		if t.holder && c.closureLiteral(t.arg) != nil {
			kind = targetLifetime
		}
		if kind == targetNone {
			continue
		}
		fl := storeFlow{target: t.arg, ref: t.target, byValue: t.holder, generic: kind == targetParam, callee: e, param: i, args: append(slotArgs(slots), args[min(len(slots), len(args)):]...)}
		params := c.linkedParams(slots, i)
		for j, v := range slots {
			if i != j && kind != targetDeferred && sharesParam(params, v.params) && ct.stores(i, j) {
				fl.values = append(fl.values, flowArg{arg: v.arg, contents: v.contents, param: j})
			}
		}
		if lit := c.closureLiteral(t.arg); t.holder && lit != nil {
			fl.values = append(c.closureFed(c.p.ByEnt[e], i, lit, fl.values), c.captureValues(lit)...)
		}
		if at := c.selfSources(e, i, slots); at != nil {
			fl.values = append(fl.values, flowArg{arg: t.arg, contents: t.contents, param: i, at: at})
		}
		fl.values = append(fl.values, c.tieValues(info, slots, t, fl.values)...)
		if kind != targetDeferred {
			fl.values = append(fl.values, c.refKeptValues(e, args, slots, i, fl.values)...)
		}
		if fl.values != nil {
			out = append(out, fl)
		}
	}
	return out
}
