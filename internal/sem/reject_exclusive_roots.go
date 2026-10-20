package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type lentRef struct {
	root, field EntityID
}

func (c *checker) aliasInit(ent EntityID) (syntax.NodeID, bool) {
	if init, ok := c.letInit[ent]; ok {
		return init, true
	}
	init, ok := c.mutAliasInit[ent]
	return init, ok
}

func (c *checker) noteMutAlias(local EntityID, init syntax.NodeID) {
	if !c.r.Types.RefMut(c.vars.resolve(c.r.Entities[local].Type)) {
		return
	}
	if c.mutAliasInit == nil {
		c.mutAliasInit = map[EntityID]syntax.NodeID{}
	}
	c.mutAliasInit[local] = init
}

// exclusiveRoots follows `let r = &mut place`, `let r = x` and branching initializers to every root the access may reach;
// lent maps a root reached through a field place to that first field, the only part of it the access owns.
func (c *checker) exclusiveRoots(root, field EntityID) ([]EntityID, map[EntityID]EntityID) {
	roots := []EntityID{root}
	var lent map[EntityID]EntityID
	if field != 0 {
		lent = map[EntityID]EntityID{root: field}
	}
	for i := 0; i < len(roots) && len(roots) < 16; i++ {
		for _, from := range c.aliasSources(roots[i]) {
			if from.root == 0 {
				continue
			}
			if from.field == 0 {
				from.field = lent[roots[i]]
			}
			if k := c.r.Entities[from.root].Kind; k != EntLocal && k != EntParam {
				continue
			}
			if slices.Contains(roots, from.root) {
				if lent[from.root] != from.field {
					delete(lent, from.root)
				}
				continue
			}
			roots = append(roots, from.root)
			if from.field != 0 {
				if lent == nil {
					lent = map[EntityID]EntityID{}
				}
				lent[from.root] = from.field
			}
		}
	}
	return roots, lent
}

func (c *checker) aliasSources(ent EntityID) []lentRef {
	if from, ok := c.lentFrom[ent]; ok && from.root != 0 {
		return []lentRef{from}
	}
	init, ok := c.aliasInit(ent)
	if !ok {
		return nil
	}
	var out []lentRef
	for _, leaf := range c.aliasLeaves(init, 0) {
		switch c.t.Kind(leaf) {
		case syntax.BorrowExpr:
			root, field := c.placeRoot(c.t.View(leaf).AsBorrow().Operand().ID())
			out = append(out, lentRef{root, field})
		case syntax.Ident:
			out = append(out, lentRef{root: c.info.Uses[leaf]})
		}
	}
	return out
}

// aliasLeaves lists every expression an initializer can evaluate to.
func (c *checker) aliasLeaves(n syntax.NodeID, depth int) []syntax.NodeID {
	if n == 0 || depth > 16 {
		return nil
	}
	n = c.t.Unparen(n)
	var parts []syntax.NodeID
	switch c.t.Kind(n) {
	case syntax.Block:
		if tail := c.t.View(n).AsBlock().Tail(); tail.Valid() {
			parts = []syntax.NodeID{tail.ID()}
		}
	case syntax.IfExpr:
		s := ifExpr(c.t, n)
		parts = []syntax.NodeID{s.Then, s.Else}
	case syntax.IfLetExpr:
		s := ifLet(c.t, n)
		parts = []syntax.NodeID{s.Then, s.Else}
	case syntax.MatchExpr:
		for arm := range c.t.View(n).AsMatch().Arms().IDs() {
			parts = append(parts, matchArm(c.t, arm).Body)
		}
	default:
		return []syntax.NodeID{n}
	}
	var out []syntax.NodeID
	for _, p := range parts {
		out = append(out, c.aliasLeaves(p, depth+1)...)
	}
	return out
}

func (c *checker) placeRoot(n syntax.NodeID) (EntityID, EntityID) {
	p, ok := c.buildPlace(n)
	if !ok {
		return 0, 0
	}
	if len(p.Fields) == 0 {
		return p.Root, 0
	}
	return p.Root, p.Fields[0]
}
