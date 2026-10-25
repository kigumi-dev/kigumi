package mir

import (
	"slices"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// lentPlace is one place a call argument may lend `&mut`, with the node of the borrow that lends it and the `&mut`
// local or call result it is reached through, if any.
type lentPlace struct {
	root  LocalID
	path  []int
	exact bool
	node  syntax.NodeID
	via   LocalID
}

// lentMutPlaces are the places a call argument may lend `&mut`, directly or through a write-back cell: a plain
// borrow's place, or every local a `&mut` that is no plain borrow may point into, also when passed as it is.
func (c *borrowCheck) lentMutPlaces(a LocalID) []lentPlace {
	for l := a; ; {
		if out := c.passedRefPlaces(l); len(out) > 0 {
			return out
		}
		d := c.definition(l)
		if c.f.Locals[l].Ent != 0 || d == nil || len(d.Args) == 0 {
			return nil
		}
		switch d.Op {
		case OpNewCell, OpShare, OpCopy, OpAlias:
			l = d.Args[0]
		case OpBorrow:
			if d.Index != 1 {
				return nil
			}
			if refs := c.storedReferents(d.Args[0]); len(refs) > 0 {
				return lentPlaces(c.heldReferents(d.Args[0], refs), d.Node, 0)
			}
			if out := c.refPlaces(d.Args[0]); len(out) > 0 {
				for i := range out {
					out[i].node = d.Node
				}
				return out
			}
			if root, path, exact, ok := c.patternPlace(d.Args[0]); ok {
				return []lentPlace{{root: root, path: path, exact: exact, node: d.Node}}
			}
			return nil
		default:
			return nil
		}
	}
}

// passedRefPlaces are the places a `&mut` holder handed over as it is may point into: a named one, or one read out
// of a stored value or returned by a call.
func (c *borrowCheck) passedRefPlaces(l LocalID) []lentPlace {
	if _, mut := c.p.R.MutRefElem(c.f.Locals[l].Type); !mut || slices.Contains(c.f.Params, l) {
		return nil
	}
	if d := c.definition(l); c.f.Locals[l].Ent == 0 && (d == nil || d.Op != OpField && d.Op != OpPayload && d.Op != OpIndex && d.Op != OpUnbox && d.Op != OpCall && d.Op != OpCallValue) {
		return nil
	}
	b, ok := c.holders[l]
	if !ok {
		return nil
	}
	if refs := c.storedReferents(l); len(refs) > 0 {
		return lentPlaces(c.heldReferents(l, refs), b.node, l)
	}
	return c.refPlaces(l)
}

func lentPlaces(refs []ownerRef, node syntax.NodeID, via LocalID) []lentPlace {
	out := make([]lentPlace, len(refs))
	for i, o := range refs {
		out[i] = lentPlace{root: o.owner, path: o.path, exact: o.exact, node: node, via: via}
	}
	return out
}

// storedReferents are the locals a `&mut` that is no plain borrow may point into: read out of a stored value, returned
// by a call (`fb.futsMut()`, `pick(&mut a, &mut b)`) or bound to a name, so a store through it may land in any of them.
func (c *borrowCheck) storedReferents(l LocalID) []ownerRef {
	b, ok := c.holders[l]
	if c.p.R.Types.Kind(c.f.Locals[l].Type) != sem.KRef || slices.Contains(c.f.Params, l) {
		return nil
	}
	if d := c.definition(l); c.f.Locals[l].Ent == 0 && (d == nil || d.Op != OpField && d.Op != OpPayload && d.Op != OpIndex && d.Op != OpCall && d.Op != OpCallValue) {
		return nil
	}
	if o, ok := c.literalPart(l); ok {
		return []ownerRef{o}
	}
	if !ok || !b.hasOwner || len(b.owners) == 0 || slices.ContainsFunc(b.owners, func(o ownerRef) bool { return !o.mut }) {
		return nil
	}
	// A holder's owners are every borrow it took, whatever field holds it; one whose type cannot hold this referent,
	// as itself or inside a field (`fa.futsMut()` borrows `fa`), is not it.
	elem := c.p.R.Types.Node(c.f.Locals[l].Type).Elem
	var out []ownerRef
	for _, o := range b.owners {
		if len(o.path) > 0 || !o.exact || c.typeHolds(c.f.Locals[o.owner].Type, func(t sem.TypeID) bool { return t == elem || c.p.R.Types.Kind(t) == sem.KParam }) {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		return b.owners
	}
	return out
}

// literalPart is the place a `&mut` read out of a record or variant literal was borrowed from, as a binding
// destructured from `Pair { a: &mut v, b: &mut w }` is.
func (c *borrowCheck) literalPart(l LocalID) (ownerRef, bool) {
	for range 8 {
		d := soleDef(c.f, l)
		if d == nil || len(d.Args) == 0 {
			return ownerRef{}, false
		}
		switch d.Op {
		case OpCopy, OpMove, OpAlias:
			l = d.Args[0]
			continue
		case OpField, OpPayload:
			lit := soleDef(c.f, d.Args[0])
			if lit == nil || lit.Op != OpRecord && lit.Op != OpVariant || d.Index >= len(lit.Args) {
				return ownerRef{}, false
			}
			b := c.definition(lit.Args[d.Index])
			if b == nil || b.Op != OpBorrow || b.Index != 1 {
				return ownerRef{}, false
			}
			root, path, exact, ok := c.patternPlace(b.Args[0])
			return ownerRef{owner: root, mut: true, path: path, exact: exact}, ok
		}
		return ownerRef{}, false
	}
	return ownerRef{}, false
}

// otherReferents are the owners past the first that a flow's target `&mut` may point into (storeRoot takes the first).
func (c *borrowCheck) otherReferents(fl storeFlow) []LocalID {
	l := fl.target
	for fl.ref && c.f.Locals[l].Ent == 0 {
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 || in.Op != OpBorrow && in.Op != OpNewCell && in.Op != OpShare && in.Op != OpCopy && in.Op != OpMove && in.Op != OpAlias {
			return nil
		}
		l = in.Args[0]
		if in.Op == OpBorrow {
			break
		}
	}
	var out []LocalID
	for i, o := range c.storedReferents(l) {
		if t := c.f.Locals[o.owner].Type; i > 0 && !c.callerPlace(o.owner) && c.p.R.Types.Kind(t) != sem.KRef {
			out = append(out, o.owner)
		}
	}
	return out
}
