package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

func (c *checker) aliasSources(ent, field EntityID) []lentRef {
	if from, ok := c.lentFrom[ent]; ok && from.root != 0 {
		return []lentRef{from.within(field)}
	}
	init, ok := c.aliasInit(ent)
	if !ok {
		return nil
	}
	out := c.leafSources(init, field, true, 0)
	for _, v := range c.held.holderAssigns[ent] {
		if !c.rebindReaches(ent, v) {
			continue
		}
		out = append(out, c.leafSources(v, field, true, 0)...)
	}
	return out
}

// holdsLentRef reports whether a value of t can carry a `&mut`, in a type argument or a declared field.
func (c *checker) holdsLentRef(t TypeID) bool {
	t = c.vars.resolve(t)
	if c.holdsMutRef(t, 0) {
		return true
	}
	n := c.r.Types.Node(t)
	if n.Kind != KNamed || c.r.Entities[n.Ent].Kind != EntType {
		return false
	}
	return slices.ContainsFunc(c.r.typeDecl(n.Ent).Fields, func(f EntityID) bool { return c.r.Types.RefMut(c.r.Entities[f].Type) })
}

// A `let mut` holder rebound later may hold what each assigned value lends.
func (c *checker) noteHolderAssign(lhs, rhs syntax.NodeID) {
	if c.t.Kind(c.t.Unparen(lhs)) != syntax.Ident {
		return
	}
	ent := c.info.Uses[c.t.Unparen(lhs)]
	if _, ok := c.mutAliasInit[ent]; !ok || c.r.Types.RefMut(c.vars.resolve(c.r.Entities[ent].Type)) {
		return
	}
	if c.held.holderAssigns == nil {
		c.held.holderAssigns = map[EntityID][]syntax.NodeID{}
	}
	c.held.holderAssigns[ent] = append(c.held.holderAssigns[ent], rhs)
}

// top marks the initializer's own leaves; a borrow found inside a literal does not inherit the holder's field.
func (c *checker) leafSources(init syntax.NodeID, field EntityID, top bool, depth int) []lentRef {
	var out []lentRef
	for _, leaf := range c.aliasLeaves(init, 0) {
		switch c.t.Kind(leaf) {
		case syntax.BorrowExpr:
			root, f := c.placeRoot(c.t.View(leaf).AsBorrow().Operand().ID())
			out = append(out, lentRef{root, f}.within(field))
		case syntax.Ident:
			if top || c.r.Types.RefMut(c.vars.resolve(c.info.Types[leaf])) {
				out = append(out, lentRef{root: c.info.Uses[leaf]}.within(field))
			}
		default:
			if parts, ok := c.containerParts(leaf, field); ok && depth < 8 {
				for _, p := range parts {
					out = append(out, c.leafSources(p, 0, false, depth+1)...)
				}
			}
		}
	}
	return out
}

func (from lentRef) within(field EntityID) lentRef {
	if from.field == 0 {
		from.field = field
	}
	return from
}

// field narrows a record literal to that entry; spreads always count.
func (c *checker) containerParts(n syntax.NodeID, field EntityID) ([]syntax.NodeID, bool) {
	switch c.t.Kind(n) {
	case syntax.TupleLit:
		return slices.Collect(c.t.List(n).IDs()), true
	case syntax.RecordLit:
		var out []syntax.NodeID
		for e := range c.t.View(n).AsRecordLit().Fields().IDs() {
			en := c.t.View(e)
			if en.Kind() == syntax.Spread || field == 0 || c.info.Uses[e] == field {
				out = append(out, en.ArgValue().ID())
			}
		}
		return out, true
	case syntax.CallExpr:
		if call, ok := c.info.Calls[n]; ok && call.Kind == CallVariant {
			return c.unwrapNamed(slices.Collect(c.t.View(n).AsCall().Args().IDs())), true
		}
	}
	return nil, false
}
