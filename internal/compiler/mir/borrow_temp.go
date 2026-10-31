package mir

import "kigumi/internal/compiler/sem"

// tempBorrow makes `&mk()`, or a pattern's borrow into it, a holder of the temporary itself,
// so a reference taken from it is traced and stays usable within its scope.
func (c *borrowCheck) tempBorrow(in Inst) (*borrowInfo, bool) {
	root, path, exact, ok := c.tempPlace(in.Args[0])
	if !ok {
		return nil, false
	}
	b := &borrowInfo{owners: []ownerRef{{owner: root, mut: in.Index == 1, path: path, exact: exact, temp: true}}, hasOwner: true, node: in.Node}
	mergeInto(b, c.throughRef(root, path, exact, in.Index == 1))
	return b, true
}

// heldPlace is the place a borrow of l takes: under a named root, else in a statement's temporary.
func (c *borrowCheck) heldPlace(l LocalID) (LocalID, stepPath, bool, bool) {
	if root, path, exact, ok := c.patternPlace(l); ok {
		return root, path, exact, true
	}
	return c.tempPlace(l)
}

func (c *borrowCheck) tempPlace(l LocalID) (LocalID, stepPath, bool, bool) {
	var path stepPath
	exact := true
	for c.f.Locals[l].Ent == 0 {
		d := c.definition(l)
		if d == nil {
			return l, path, exact, c.multiDefined(l) && c.joinsBuilt(l)
		}
		switch d.Op {
		case OpCall, OpCallValue, OpRecord, OpVariant, OpArray:
			return l, path, exact, true
		case OpField, OpPayload, OpIndex, OpUnbox:
			path, exact = path.under(d, exact, nil)
		case OpCopy, OpBorrow:
		default:
			return 0, nil, false, false
		}
		l = d.Args[0]
	}
	return 0, nil, false, false
}

// joinsBuilt reports whether every definition of l builds a fresh value, or copies a temporary that does,
// as the arms of `x?.f`, `if` and `match` do.
func (c *borrowCheck) joinsBuilt(l LocalID) bool {
	return c.builtFresh(l, map[LocalID]bool{})
}

func (c *borrowCheck) builtFresh(l LocalID, seen map[LocalID]bool) bool {
	if seen[l] {
		return true
	}
	seen[l] = true
	defined := false
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if !shapes[in.Op].result || in.Dst != l {
				continue
			}
			defined = true
			switch in.Op {
			case OpCall, OpCallValue, OpRecord, OpVariant, OpArray:
			case OpCopy:
				if c.f.Locals[in.Args[0]].Ent != 0 || !c.builtFresh(in.Args[0], seen) {
					return false
				}
			default:
				return false
			}
		}
	}
	return defined
}

// checkTempKept rejects binding a name to a borrow of a statement's temporary, also one a Future or Task captured.
func (c *borrowCheck) checkTempKept(in Inst) {
	if in.Op != OpCopy && in.Op != OpMove || c.f.Locals[in.Dst].Ent == 0 {
		return
	}
	if t := c.f.Locals[in.Dst].Type; !c.resultMayCarryLifetime(t) && !c.holdsAsync(t) && !c.namesTypeParam(t) {
		return
	}
	b, ok := c.holders[in.Args[0]]
	if def := c.definition(in.Args[0]); !ok || def != nil && def.Op == OpBorrow {
		return
	}
	if c.keepsTemp(b) {
		c.reportCode(in.Node, in.Dst, "temporary-borrow-kept", c.f.Locals[in.Dst].Name, tempDropped)
		c.keptTemp[in.Dst] = true
		c.noteKept(b)
	}
}

func (c *borrowCheck) keepsTemp(b *borrowInfo) bool {
	for _, o := range b.owners {
		if o.temp && !c.scalarTemp(o.owner) {
			return true
		}
	}
	return false
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
