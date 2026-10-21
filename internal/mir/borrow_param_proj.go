package mir

import (
	"slices"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// projectionLifetime is the parameter lifetime a borrow of place l under root lives for: all of a
// reference's (or elided receiver's) referent, or only what a lifetime-typed value reaches through a reference.
func (c *borrowCheck) projectionLifetime(root, l LocalID) sem.EntityID {
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

// referentLifetime is the one parameter lifetime every owner of b was borrowed
// under, 0 when some owner is a local of this function.
func referentLifetime(b *borrowInfo) sem.EntityID {
	var lt sem.EntityID
	for _, o := range b.owners {
		if o.lifetime == 0 || lt != 0 && o.lifetime != lt {
			return 0
		}
		lt = o.lifetime
	}
	return lt
}

type recvBorrowKey struct {
	arg  LocalID
	mut  bool
	node syntax.NodeID
}

// receiverPlaceInfo is the borrow a call whose result takes its receiver's
// elided lifetime makes of the receiver's place, cached per argument.
func (c *borrowCheck) receiverPlaceInfo(a LocalID, mut bool) (*borrowInfo, bool) {
	key := recvBorrowKey{a, mut, c.callNode}
	if b, ok := c.recvBorrows[key]; ok {
		return b, true
	}
	if d := c.definition(a); d != nil && d.Op == OpNewCell {
		a = d.Args[0]
	}
	root, path, exact, ok := c.patternPlace(a)
	if !ok {
		return nil, false
	}
	b := &borrowInfo{owners: []ownerRef{{owner: root, mut: mut, path: path, exact: exact, lifetime: c.projectionLifetime(root, a)}}, hasOwner: true, node: c.callNode}
	if c.recvBorrows == nil {
		c.recvBorrows = map[recvBorrowKey]*borrowInfo{}
	}
	c.recvBorrows[key] = b
	return b, true
}
