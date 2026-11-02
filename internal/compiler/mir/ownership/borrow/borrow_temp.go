package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// tempBorrow makes `&mk()`, or a pattern's borrow into it, a holder of the temporary itself,
// so a reference taken from it is traced and stays usable within its scope.
func (c *borrowCheck) tempBorrow(in midir.Inst) (*borrowInfo, bool) {
	root, path, exact, ok := c.tempPlace(in.Args[0])
	if !ok {
		return nil, false
	}
	b := &borrowInfo{owners: []ownerRef{{owner: root, mut: in.Index == 1, path: path, exact: exact, temp: true}}, hasOwner: true, node: in.Node}
	mergeInto(b, c.throughRef(root, path, exact, in.Index == 1))
	return b, true
}

// heldPlace is the place a borrow of l takes: under a named root, else in a statement's temporary.
func (c *borrowCheck) heldPlace(l midir.LocalID) (midir.LocalID, ownquery.Path, bool, bool) {
	if root, path, exact, ok := c.patternPlace(l); ok {
		return root, path, exact, true
	}
	return c.tempPlace(l)
}

// tempPlace: a value ends with its statement unless its definition proves it a read of a longer-lived place
// (a named local, a static, a cell's content, an iterated element) or an immortal literal.
func (c *borrowCheck) tempPlace(l midir.LocalID) (midir.LocalID, ownquery.Path, bool, bool) {
	var path ownquery.Path
	exact := true
	for c.f.Locals[l].Ent == 0 {
		d := c.definition(l)
		if d == nil {
			return l, path, exact, c.multiDefined(l) && c.ownsValue(l)
		}
		switch {
		case d.Op == midir.OpField, d.Op == midir.OpPayload, d.Op == midir.OpIndex, d.Op == midir.OpUnbox:
			path, exact = path.Under(d, exact, nil)
		case d.Op == midir.OpCopy, d.Op == midir.OpBorrow, d.Op == midir.OpShare, d.Op == midir.OpAlias, d.Op == midir.OpCellGet:
		case d.Op == midir.OpMove && c.f.Locals[d.Args[0]].Ent == 0:
		case iterElement(d):
			path, exact = nil, false
		case d.Op == midir.OpGlobalGet, d.Op == midir.OpUnit, d.Op == midir.OpCFnPtr, d.Op == midir.OpLendBox, immortalLiteral(d):
			return 0, nil, false, false
		case d.Op == midir.OpCall, d.Op == midir.OpCallValue, d.Op == midir.OpRecord, d.Op == midir.OpVariant, d.Op == midir.OpArray, d.Op == midir.OpMove:
			return l, path, exact, true
		default:
			return l, path, exact, c.ownsValue(l)
		}
		l = d.Args[0]
	}
	return 0, nil, false, false
}

// immortalLiteral: llgen gives scalar and string literals a static slot and the VM interns them alike;
// a Bytes literal (erased in place) and a record constant are built afresh at every use.
func immortalLiteral(d *midir.Inst) bool {
	switch d.Lit.Kind {
	case sem.LitInt, sem.LitFloat, sem.LitChar, sem.LitBool, sem.LitString:
		return d.Op == midir.OpConst
	}
	return false
}

// ownsValue: a reference or a scalar holds nothing its end frees.
func (c *borrowCheck) ownsValue(l midir.LocalID) bool {
	return c.p.R.Types.Kind(c.f.Locals[l].Type) != sem.KRef && !c.scalarTemp(l)
}

// checkTempKept rejects binding a name to a borrow of a statement's temporary, also one a Future or Task captured.
func (c *borrowCheck) checkTempKept(in midir.Inst) {
	if in.Op != midir.OpCopy && in.Op != midir.OpMove || c.f.Locals[in.Dst].Ent == 0 {
		return
	}
	if t := c.f.Locals[in.Dst].Type; !c.resultMayCarryLifetime(t) && !c.holdsAsync(t) && !c.facts().NamesTypeParam(t) {
		return
	}
	b, ok := c.holders[in.Args[0]]
	if def := c.definition(in.Args[0]); !ok || def != nil && def.Op == midir.OpBorrow {
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

func (c *borrowCheck) scalarTemp(l midir.LocalID) bool {
	t := c.f.Locals[l].Type
	return c.p.R.Types.Kind(t) == sem.KPrim && t != sem.TyString && t != sem.TyBytes
}

// tempScopeEnd reports a scope-owned temporary that ends while a named holder still borrows it;
// a pattern binding into it dies with the same scope, and checkTempKept already named a kept one.
func (c *borrowCheck) tempScopeEnd(in midir.Inst, holder midir.LocalID, b *borrowInfo, o ownerRef) bool {
	if c.f.Locals[o.owner].Ent != 0 {
		return false
	}
	if in.Dst == o.owner {
		return true
	}
	if in.Op != midir.OpDrop || !in.EndScope {
		return false
	}
	if c.f.Locals[holder].Ent != 0 && !c.keptTemp[holder] && !c.boundByBorrow(holder) {
		c.reportCode(b.node, holder, "temporary-borrow-kept", c.f.Locals[holder].Name, tempDropped)
	}
	return true
}

func (c *borrowCheck) boundByBorrow(l midir.LocalID) bool {
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			if in.Dst == l && in.Op.Yields() {
				return in.Op == midir.OpBorrow
			}
		}
	}
	return false
}
