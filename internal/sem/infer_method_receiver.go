package sem

import "kigumi/internal/syntax"

func (c *checker) takeReceiver(n, base syntax.NodeID, recv TypeID, viaRef bool) bool {
	b := c.t.Unparen(base)
	if viaRef || c.lentRoot(b) {
		c.errAt(n, cMethodValueBorrowed, recv)
		return false
	}
	switch c.t.Kind(b) {
	case syntax.Ident:
		if ent := c.info.Uses[b]; ent != 0 {
			if k := c.r.Entities[ent].Kind; k == EntLocal || k == EntParam {
				return true
			}
		}
		c.errAt(n, cMethodValuePlace, recv, "a static", "bind the value to a local first, or call the method directly")
		return false
	case syntax.MemberExpr:
		return c.movableField(n, b, recv)
	case syntax.BracketExpr:
		c.errAt(n, cMethodValuePlace, recv, "an element, since indexing never moves a value out", "take the element out with `.takeAt` first, or call the method directly")
		return false
	case syntax.OptMemberExpr:
		c.errAt(n, cMethodValuePlace, recv, "a field read through `?.`", "bind the value to a local first, or call the method directly")
		return false
	}
	return true
}

func (c *checker) movableField(n, field syntax.NodeID, recv TypeID) bool {
	if c.info.Uses[field] == 0 || c.r.Entities[c.info.Uses[field]].Kind != EntField {
		c.errAt(n, cMethodValuePlace, recv, "this expression", "bind the value to a local first, or call the method directly")
		return false
	}
	container := c.readThrough(c.vars.resolve(c.info.Types[c.t.View(field).Base().ID()]))
	if bn := c.r.Types.Node(container); bn.Kind == KNamed {
		if c.isSharedEnt(bn.Ent) {
			c.errAt(n, cMethodValuePlace, recv, "`"+c.placeName(field)+"` inside a `Shared`", "borrow the content and call the method directly")
			return false
		}
		if c.r.typeDecl(bn.Ent).Drop != 0 {
			c.errAt(n, cMethodValueDropField, recv, c.placeName(field), container, container)
			return false
		}
	}
	for b := c.t.View(field).Base().ID(); ; {
		switch c.t.Kind(b) {
		case syntax.Paren, syntax.MemberExpr:
			b = c.t.View(b).Base().ID()
			continue
		case syntax.Ident:
			if ent := c.info.Uses[b]; ent != 0 {
				if k := c.r.Entities[ent].Kind; k == EntLocal || k == EntParam {
					if root, aliased := c.methodValues.aliases[ent]; aliased {
						name := c.r.Entities[root].Name
						c.errAt(n, cMethodValuePlace, recv, "a field of `"+c.r.Entities[ent].Name+"`, which aliases the matched `"+name+"`", "destructure `"+name+"` with `let ... else` to give it up, or call the method directly")
						return false
					}
					if !c.capturedHere(ent) {
						c.noteFieldMoveSite(ent, n, field, recv)
						return true
					}
					c.errAt(n, cMethodValuePlace, recv, "a field of `"+c.r.Entities[ent].Name+"`, which this closure captured and may run again", "move the field into a local before the closure, or call the method directly")
					return false
				}
			}
			c.errAt(n, cMethodValuePlace, recv, "a field of a static", "bind the value to a local first, or call the method directly")
		case syntax.BracketExpr:
			c.errAt(n, cMethodValuePlace, recv, "a field of an element, since indexing never moves a value out", "take the element out with `.takeAt` first, or call the method directly")
		case syntax.OptMemberExpr:
			c.errAt(n, cMethodValuePlace, recv, "a field of a temporary read through `?.`", "bind the value to a local first, or call the method directly")
		default:
			c.errAt(n, cMethodValuePlace, recv, "a field of a temporary", "bind the temporary to a local first, or call the method directly")
		}
		return false
	}
}

func (c *checker) lentRoot(n syntax.NodeID) bool {
	tt := c.r.Types
	for i := 0; n != 0 && i < 64; i++ {
		if tt.Kind(c.vars.resolve(c.info.Types[n])) == KRef {
			return true
		}
		switch c.t.Kind(n) {
		case syntax.Paren:
			n = c.t.View(n).AsParen().Inner().ID()
		case syntax.MemberExpr, syntax.BracketExpr, syntax.OptMemberExpr:
			n = c.t.View(n).Base().ID()
		case syntax.Ident:
			ent := c.info.Uses[n]
			if ent == 0 {
				return false
			}
			e := &c.r.Entities[ent]
			return e.Flags&EfSelf != 0 && e.Flags&EfMove == 0 || tt.Kind(e.Type) == KRef
		default:
			return false
		}
	}
	return false
}
