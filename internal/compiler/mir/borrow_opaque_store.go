package mir

import "kigumi/internal/compiler/sem"

// checkOpaqueStores rejects handing a borrow to a function value next to a
// `&mut` place that names a lifetime: the callee's writes cannot be followed.
func (c *borrowCheck) checkOpaqueStores(in Inst) {
	c.checkRelayStores(in)
	if in.Op != OpCallValue || in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn || len(in.Args) < 2 {
		return
	}
	known := len(c.summarizedValueCallees(in)) > 0
	for i, t := range in.Args[1:] {
		typ := c.f.Locals[t].Type
		if c.p.R.Types.Kind(typ) != sem.KRef || !c.p.R.Types.RefMut(typ) || c.declaredLifetime(typ) == 0 {
			continue
		}
		if known && c.keptPlace(t) {
			continue
		}
		for j, v := range in.Args[1:] {
			if i == j {
				continue
			}
			if b, ok := c.flowInfo(flowArg{arg: v}); ok && !b.empty {
				name := c.f.Locals[t].Name
				if root, _, _, ok := c.storeRoot(storeFlow{target: t, ref: true}); ok {
					name = c.f.Locals[root].Name
				}
				c.reportCode(in.Node, v, "borrow-store-opaque-call", name)
				return
			}
		}
	}
}

// opaqueFlows let a call through a function value store what each argument holds into every `&mut` argument and
// every `&mut` holder handed over by value.
func (c *borrowCheck) opaqueFlows(in Inst) []storeFlow {
	if in.Op != OpCallValue || in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn || len(in.Args) < 2 {
		return nil
	}
	args := in.Args[1:]
	known := len(c.summarizedValueCallees(in)) > 0
	var out []storeFlow
	for i, t := range args {
		byValue := c.holdsMutPlace(t)
		if _, mut := c.p.R.MutRefElem(c.f.Locals[t].Type); !mut && !byValue {
			continue
		}
		if known && c.keptPlace(t) {
			continue
		}
		lifetime := c.declaredLifetime(c.f.Locals[t].Type) != 0 || c.declaredLifetime(c.placeOf(t)) != 0
		if !lifetime && !c.namesTypeParam(c.f.Locals[t].Type) && !c.namesTypeParam(c.placeOf(t)) {
			continue
		}
		fl := storeFlow{target: t, ref: !byValue, byValue: byValue, generic: !lifetime, opaque: true}
		for j, v := range args {
			contents := c.p.R.Types.Kind(c.f.Locals[v].Type) == sem.KRef
			if j != i {
				fl.values = append(fl.values, flowArg{arg: v, contents: contents})
			} else {
				fl.values = append(fl.values, flowArg{arg: v, contents: contents, at: []stepPath{{}}})
			}
		}
		out = append(out, fl)
	}
	if c.capturesHolder(in.Args[0]) {
		fl := storeFlow{target: in.Args[0], byValue: true, opaque: true, values: c.captureValues(c.closureLiteral(in.Args[0]))}
		for _, v := range args {
			fl.values = append(fl.values, flowArg{arg: v, contents: c.p.R.Types.Kind(c.f.Locals[v].Type) == sem.KRef})
		}
		out = append(out, fl)
	}
	return out
}
