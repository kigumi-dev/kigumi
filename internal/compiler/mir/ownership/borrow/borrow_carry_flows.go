package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// carryFlows lists, per fn call in runs, its `&mut` targets and holders with the by-value arguments carrying a borrow
// their types do not name (a Future or Task) the fn may put there; a type parameter's borrow goes through callFlows.
func (c *borrowCheck) carryFlows(in midir.Inst) []storeFlow {
	var out []storeFlow
	for _, t := range c.flowCallees(in) {
		out = append(out, c.targetCarry(t)...)
	}
	return out
}

func (c *borrowCheck) targetCarry(ct callTarget) []storeFlow {
	info := c.p.R.Fn(ct.ent)
	var targets []storeFlow
	var values []flowArg
	var stores []func(int) bool
	for i, s := range c.callSlots(info, ct.args, 0) {
		if s.target || s.holder {
			targets = append(targets, storeFlow{target: s.arg, ref: s.target, byValue: s.holder, callee: ct.ent, param: i})
			stores = append(stores, ct.storesInto(i, c.holderLiteral(s) != nil))
			continue
		}
		if i == 0 && info.Recv != sem.RecvNone || c.p.R.Types.Kind(s.exact) == sem.KRef {
			continue
		}
		t := c.f.Locals[s.arg].Type
		if _, ok := c.holders[s.arg]; ok && !c.resultMayCarryLifetime(t) && (c.holdsDeferred(t) || !c.facts().NamesTypeParam(t)) {
			values = append(values, flowArg{arg: s.arg, param: i})
		}
	}
	var out []storeFlow
	for k, fl := range targets {
		for _, v := range values {
			if stores[k](v.param) {
				fl.values = append(fl.values, flowArg{arg: v.arg})
			}
		}
		if len(fl.values) > 0 {
			out = append(out, fl)
		}
	}
	return out
}
