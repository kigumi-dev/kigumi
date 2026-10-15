package sem

import "kigumi/internal/syntax"

// Matches on the declaring package since langItems has no entry for Shared
// (only effects_drop.go's dead check reads one).
func (c *checker) isSharedEnt(ent EntityID) bool {
	e := &c.r.Entities[ent]
	return e.Name == "Shared" && c.r.Packages[e.Pkg].Path == "std/alloc"
}

// Forces an owned read of a field path that would otherwise bind as a borrow,
// so the declared type comes back, not `&T`.
func (c *checker) synthMove(n syntax.NodeID) TypeID {
	target := c.t.Unparen(c.t.View(n).AsMove().Operand().ID())
	isField := c.t.Kind(target) == syntax.MemberExpr
	var baseType TypeID
	if isField {
		base := c.t.View(target).AsMember().Base().ID()
		baseType = c.readThrough(c.vars.resolve(c.synth(base)))
		if bn := c.r.Types.Node(baseType); bn.Kind == KNamed && c.isSharedEnt(bn.Ent) {
			c.errAt(target, cMoveShared, baseType)
			return TyPoison
		}
	}
	t := c.synthPlace(target)
	if t == TyPoison {
		return TyPoison
	}
	if c.indexRooted(target) {
		c.errAt(target, cMoveIndex)
		return t
	}
	p, ok := c.buildPlace(target)
	if !ok {
		return t
	}
	if p.Deref {
		c.errAt(target, cMoveOutOfBorrow, t)
		return t
	}
	if !isField {
		return t
	}
	if bn := c.r.Types.Node(baseType); bn.Kind == KNamed {
		if info := c.r.typeDecl(bn.Ent); info.Drop != 0 {
			name := c.placeName(target)
			c.errAt(target, cMoveDropType, name, baseType, name)
		}
	}
	return t
}
