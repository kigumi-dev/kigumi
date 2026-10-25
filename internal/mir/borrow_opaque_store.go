package mir

import "kigumi/internal/sem"

// checkOpaqueStores rejects handing a borrow to a function value next to a
// `&mut` place that names a lifetime: the callee's writes cannot be followed.
func (c *borrowCheck) checkOpaqueStores(in Inst) {
	if in.Op != OpCallValue || in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn || len(in.Args) < 2 {
		return
	}
	for i, t := range in.Args[1:] {
		typ := c.f.Locals[t].Type
		if c.p.R.Types.Kind(typ) != sem.KRef || !c.p.R.Types.RefMut(typ) || c.declaredLifetime(typ) == 0 {
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

// opaqueFlows let a call through a function value store what each argument holds into every `&mut` argument.
func (c *borrowCheck) opaqueFlows(in Inst) []storeFlow {
	if in.Op != OpCallValue || in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn || len(in.Args) < 2 {
		return nil
	}
	args := in.Args[1:]
	var out []storeFlow
	for i, t := range args {
		if _, mut := c.p.R.MutRefElem(c.f.Locals[t].Type); !mut {
			continue
		}
		lifetime := c.declaredLifetime(c.f.Locals[t].Type) != 0 || c.declaredLifetime(c.placeOf(t)) != 0
		if !lifetime && !c.namesTypeParam(c.f.Locals[t].Type) && !c.namesTypeParam(c.placeOf(t)) {
			continue
		}
		fl := storeFlow{target: t, ref: true, generic: !lifetime, opaque: true}
		for j, v := range args {
			if j != i {
				fl.values = append(fl.values, flowArg{arg: v, contents: c.p.R.Types.Kind(c.f.Locals[v].Type) == sem.KRef})
			}
		}
		out = append(out, fl)
	}
	return out
}
