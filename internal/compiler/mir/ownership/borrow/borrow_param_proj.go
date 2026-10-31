package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// projectionLifetime is the parameter lifetime a borrow of place l under root lives for: all of a
// reference's (or elided receiver's) referent, or only what a lifetime-typed value reaches through a reference.
func (c *borrowCheck) projectionLifetime(root, l midir.LocalID) sem.EntityID {
	isRef := c.p.R.Types.Kind(c.f.Locals[root].Type) == sem.KRef
	if !isRef && !slices.Contains(c.f.Params, root) {
		return 0
	}
	b, ok := c.holders[root]
	if !ok || b.hasOwner || b.lifetime == 0 || b.multi != 0 {
		return 0
	}
	ent := c.f.Locals[root].Ent
	if isRef || ent != 0 && c.p.R.Local(ent).Lifetime != 0 {
		return b.lifetime
	}
	for l != root {
		if c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef {
			return b.lifetime
		}
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 {
			return 0
		}
		l = in.Args[0]
	}
	return 0
}

// referentLifetime is the one parameter lifetime every owner of b was borrowed under, 0 when some owner is a
// local of this function; a reference local reborrowed through stands for what it borrows, also an owner of b.
func referentLifetime(b *borrowInfo) sem.EntityID {
	var lt sem.EntityID
	for _, o := range b.owners {
		if o.through && o.lifetime == 0 {
			continue
		}
		if o.lifetime == 0 || lt != 0 && o.lifetime != lt {
			return 0
		}
		lt = o.lifetime
	}
	return lt
}

type recvBorrowKey struct {
	arg  midir.LocalID
	mut  bool
	node syntax.NodeID
}

// receiverPlaceInfo is the borrow a call whose result takes its receiver's elided lifetime makes of the
// receiver's place, and through a reference place also of what it borrows, cached per argument.
func (c *borrowCheck) receiverPlaceInfo(a midir.LocalID, mut bool) (*borrowInfo, bool) {
	key := recvBorrowKey{a, mut, c.callNode}
	if b, ok := c.recvBorrows[key]; ok {
		return b, true
	}
	if d := c.definition(a); d != nil && d.Op == midir.OpNewCell {
		a = d.Args[0]
	}
	if d := c.definition(a); d != nil && d.Op == midir.OpShare {
		a = d.Args[0]
	}
	root, path, exact, ok := c.patternPlace(a)
	if !ok {
		return nil, false
	}
	through := c.p.R.Types.Kind(c.f.Locals[root].Type) == sem.KRef
	b := &borrowInfo{owners: []ownerRef{{owner: root, mut: mut, path: path, exact: exact, lifetime: c.projectionLifetime(root, a), through: through}}, hasOwner: true, node: c.callNode}
	site := reborrowSite{b, root, path, exact, mut}
	c.reborrows = append(c.reborrows, site)
	site.refresh(c)
	if c.recvBorrows == nil {
		c.recvBorrows = map[recvBorrowKey]*borrowInfo{}
	}
	c.recvBorrows[key] = b
	return b, true
}
