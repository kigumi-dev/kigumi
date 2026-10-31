package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

func (c *checker) aliasSources(ent, field EntityID) []lentRef {
	if from, ok := c.lentFrom[ent]; ok && from.root != 0 {
		return []lentRef{from.within(field)}
	}
	out := c.partAssignSources(ent, field)
	init, ok := c.aliasInit(ent)
	if !ok {
		return out
	}
	out = append(out, c.leafSources(init, field, true, 0)...)
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
	return c.carriesLentRef(t, 0)
}

func (c *checker) carriesLentRef(t TypeID, depth int) bool {
	t = c.vars.resolve(t)
	if c.r.Types.RefMut(t) {
		return true
	}
	if depth >= 8 {
		return false
	}
	n := c.r.Types.Node(t)
	if slices.ContainsFunc(n.Args, func(a TypeID) bool { return c.carriesLentRef(a, depth+1) }) {
		return true
	}
	if n.Kind != KNamed || c.r.Entities[n.Ent].Kind != EntType {
		return false
	}
	return slices.ContainsFunc(c.r.typeDecl(n.Ent).Fields, func(f EntityID) bool { return c.carriesLentRef(c.r.Entities[f].Type, depth+1) })
}

func (c *checker) holdsLentHolder(t TypeID) bool {
	return c.carriesLentHolder(t, 0)
}

func (c *checker) carriesLentHolder(t TypeID, depth int) bool {
	t = c.vars.resolve(t)
	n := c.r.Types.Node(t)
	if c.r.Types.RefMut(t) {
		return c.holdsLentRef(n.Elem)
	}
	if depth >= 8 {
		return false
	}
	if slices.ContainsFunc(n.Args, func(a TypeID) bool { return c.carriesLentHolder(a, depth+1) }) {
		return true
	}
	if n.Kind != KNamed || c.r.Entities[n.Ent].Kind != EntType {
		return false
	}
	return slices.ContainsFunc(c.r.typeDecl(n.Ent).Fields, func(f EntityID) bool { return c.carriesLentHolder(c.r.Entities[f].Type, depth+1) })
}

// A `let mut` holder rebound later may hold what each assigned value lends.
func (c *checker) noteHolderAssign(lhs, rhs syntax.NodeID) {
	if c.t.Kind(c.t.Unparen(lhs)) != syntax.Ident {
		c.notePartAssign(lhs, rhs)
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
			if top || c.holdsLentRef(c.info.Types[leaf]) {
				out = append(out, lentRef{root: c.info.Uses[leaf]}.within(field))
			}
		case syntax.MemberExpr, syntax.BracketExpr:
			if root, f := c.placeRoot(leaf); root != 0 && c.holdsLentRef(c.info.Types[leaf]) {
				out = append(out, lentRef{root, f}.within(field))
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
		args := c.unwrapNamed(slices.Collect(c.t.View(n).AsCall().Args().IDs()))
		call, ok := c.info.Calls[n]
		if ok && call.Kind == CallVariant {
			return args, true
		}
		if !c.holdsLentRef(c.info.Types[n]) {
			return nil, false
		}
		if callee := c.t.View(n).AsCall().Callee(); ok && call.Kind == CallMethod && callee.Kind() == syntax.MemberExpr {
			args = append(args, callee.AsMember().Base().ID())
		}
		return args, true
	case syntax.Spread:
		return []syntax.NodeID{c.t.View(n).AsSpread().Value().ID()}, true
	}
	return nil, false
}
