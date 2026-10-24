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
	if from, ok := c.held.partOf[ent]; ok {
		return from, true
	}
	init, ok := c.mutAliasInit[ent]
	return init, ok
}

func (c *checker) noteMutAlias(local EntityID, init syntax.NodeID) {
	if t := c.r.Entities[local].Type; !c.r.Types.RefMut(c.vars.resolve(t)) && !c.holdsLentRef(t) {
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
	return c.rootsFrom([]lentRef{{root, field}})
}

func (c *checker) rootsFrom(start []lentRef) ([]EntityID, map[EntityID]EntityID) {
	var roots []EntityID
	var lent map[EntityID]EntityID
	add := func(from lentRef) {
		if slices.Contains(roots, from.root) {
			if lent[from.root] != from.field {
				delete(lent, from.root)
			}
			return
		}
		roots = append(roots, from.root)
		if from.field != 0 {
			if lent == nil {
				lent = map[EntityID]EntityID{}
			}
			lent[from.root] = from.field
		}
	}
	for _, from := range start {
		add(from)
	}
	for i := 0; i < len(roots) && len(roots) < 16; i++ {
		for _, from := range c.aliasSources(roots[i], lent[roots[i]]) {
			if from.root == 0 {
				continue
			}
			if k := c.r.Entities[from.root].Kind; k != EntLocal && k != EntParam {
				continue
			}
			add(from)
		}
	}
	return roots, lent
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
