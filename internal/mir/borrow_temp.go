package mir

import "kigumi/internal/sem"

// tempBorrow makes `&mk()`, or a pattern's borrow into it, a holder of the temporary itself,
// so a reference taken from it is traced and stays usable within its scope.
func (c *borrowCheck) tempBorrow(in Inst) (*borrowInfo, bool) {
	root, path, exact, ok := c.tempPlace(in.Args[0])
	if !ok {
		return nil, false
	}
	return &borrowInfo{owners: []ownerRef{{owner: root, mut: in.Index == 1, path: path, exact: exact, temp: true}}, hasOwner: true, node: in.Node}, true
}

func (c *borrowCheck) tempPlace(l LocalID) (LocalID, []int, bool, bool) {
	var path []int
	exact := true
	for c.f.Locals[l].Ent == 0 {
		d := c.definition(l)
		if d == nil {
			return 0, nil, false, false
		}
		switch d.Op {
		case OpCall, OpCallValue, OpRecord, OpVariant, OpArray:
			return l, path, exact, true
		case OpField:
			path = append([]int{d.Index}, path...)
		case OpPayload:
			path = append([]int{payloadStep(d.Index)}, path...)
		case OpIndex, OpUnbox:
			path, exact = nil, false
		case OpCopy, OpBorrow:
		default:
			return 0, nil, false, false
		}
		l = d.Args[0]
	}
	return 0, nil, false, false
}

// checkTempKept rejects binding a name to a borrow of a statement's temporary.
func (c *borrowCheck) checkTempKept(in Inst) {
	if in.Op != OpCopy && in.Op != OpMove || c.f.Locals[in.Dst].Ent == 0 || !c.resultMayCarryLifetime(c.f.Locals[in.Dst].Type) {
		return
	}
	b, ok := c.holders[in.Args[0]]
	if def := c.definition(in.Args[0]); !ok || def != nil && def.Op == OpBorrow {
		return
	}
	for _, o := range b.owners {
		if o.temp && !c.scalarTemp(o.owner) {
			c.reportCode(in.Node, in.Dst, "temporary-borrow-kept", c.f.Locals[in.Dst].Name, tempDropped)
			c.keptTemp[in.Dst] = true
			return
		}
	}
}

const tempDropped = "past the statement, `match` or `if` that drops it"

func (c *borrowCheck) scalarTemp(l LocalID) bool {
	t := c.f.Locals[l].Type
	return c.p.R.Types.Kind(t) == sem.KPrim && t != sem.TyString && t != sem.TyBytes
}

// tempScopeEnd reports a scope-owned temporary that ends while a named holder still borrows it;
// a pattern binding into it dies with the same scope, and checkTempKept already named a kept one.
func (c *borrowCheck) tempScopeEnd(in Inst, holder LocalID, b *borrowInfo, o ownerRef) bool {
	if c.f.Locals[o.owner].Ent != 0 {
		return false
	}
	if in.Dst == o.owner {
		return true
	}
	if in.Op != OpDrop || !in.EndScope {
		return false
	}
	if c.f.Locals[holder].Ent != 0 && !c.keptTemp[holder] && !c.boundByBorrow(holder) {
		c.reportCode(b.node, holder, "temporary-borrow-kept", c.f.Locals[holder].Name, tempDropped)
	}
	return true
}

func (c *borrowCheck) boundByBorrow(l LocalID) bool {
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if in.Dst == l && shapes[in.Op].result {
				return in.Op == OpBorrow
			}
		}
	}
	return false
}
