package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) placeOf(n syntax.NodeID) PlaceID {
	if id, ok := c.info.Places[n]; ok {
		return id
	}
	p, ok := c.buildPlace(n)
	if !ok {
		return 0
	}
	id := c.r.internPlace(p)
	c.info.Places[n] = id
	return id
}

func (c *checker) buildPlace(n syntax.NodeID) (Place, bool) {
	node := c.t.View(n)
	switch node.Kind() {
	case syntax.Ident:
		ent := c.info.Uses[n]
		if ent == 0 {
			if b, _, ok := c.r.lookup(c.scope, node.Tok().Text()); ok {
				ent = b.Ent
			}
		}
		if ent == 0 {
			return Place{}, false
		}
		e := &c.r.Entities[ent]
		if e.Kind == EntContextKey {
			return c.ctxPlace(n)
		}
		if e.Kind != EntLocal && e.Kind != EntParam {
			return Place{}, false
		}
		return c.r.identPlace(ent), true
	case syntax.Paren:
		return c.buildPlace(node.AsParen().Inner().ID())
	case syntax.MemberExpr:
		if ent := c.info.Uses[n]; ent != 0 && c.r.Entities[ent].Kind == EntContextKey {
			return c.ctxPlace(n)
		}
		base, ok := c.buildPlace(node.AsMember().Base().ID())
		if !ok {
			return Place{}, false
		}
		fld := c.info.Uses[n]
		if fld != 0 && c.r.Entities[fld].Kind == EntType {
			// `.value`: the same storage as the receiver, just retyped.
			return base, true
		}
		if fld == 0 || c.r.Entities[fld].Kind != EntField {
			return Place{}, false
		}
		return c.r.fieldPlace(base, fld), true
	case syntax.BracketExpr:
		base, ok := c.buildPlace(node.AsBracket().Base().ID())
		if !ok {
			return Place{}, false
		}
		return Place{Root: base.Root, Fields: base.Fields, Index: true, Deref: base.Deref, Mutable: base.Mutable}, true
	}
	return Place{}, false
}

// only Array[T] and FixedArray[T, N] have an index-set (E617).
func (c *checker) indexAssignUnsupported(n syntax.NodeID) bool {
	switch c.t.Kind(n) {
	case syntax.Paren, syntax.MemberExpr:
		return c.indexAssignUnsupported(c.t.View(n).Base().ID())
	case syntax.BracketExpr:
		base := c.t.View(n).AsBracket().Base().ID()
		t := c.vars.resolve(c.info.Types[base])
		if t == 0 || t == TyPoison {
			return false
		}
		tt := c.r.Types
		if tt.Kind(t) == KRef {
			t = tt.Node(t).Elem
		}
		if !(tt.Kind(t) == KNamed && tt.Node(t).Ent == tt.arrayEnt) && !c.r.isFixedArray(t) {
			return true
		}
		return c.indexAssignUnsupported(base)
	}
	return false
}

// isMutablePlace: every link of the place chain must be mutable.
func (c *checker) isMutablePlace(n syntax.NodeID) bool {
	p, ok := c.buildPlace(n)
	return ok && p.Mutable
}

func (c *checker) isPlace(n syntax.NodeID) bool {
	_, ok := c.buildPlace(n)
	return ok
}

// Applies only to a local/parameter named directly, not a field or element.
func (c *checker) isIdentPlace(n syntax.NodeID) bool {
	_, ok := c.identPlaceEnt(n)
	return ok
}

func (c *checker) identPlaceEnt(n syntax.NodeID) (EntityID, bool) {
	for {
		switch {
		case c.t.Kind(n) == syntax.Paren:
			n = c.t.View(n).AsParen().Inner().ID()
			continue
		case c.t.Kind(n) == syntax.MemberExpr:
			// `.value` is the same storage as its receiver: peel through it
			// the way Paren is peeled.
			if use := c.info.Uses[n]; use != 0 && c.r.Entities[use].Kind == EntType {
				n = c.t.View(n).AsMember().Base().ID()
				continue
			}
		}
		break
	}
	if c.t.Kind(n) != syntax.Ident {
		return 0, false
	}
	ent := c.info.Uses[n]
	if ent == 0 || (c.r.Entities[ent].Kind != EntLocal && c.r.Entities[ent].Kind != EntParam) {
		return 0, false
	}
	return ent, true
}

func (c *checker) placeName(n syntax.NodeID) string {
	node := c.t.View(n)
	switch node.Kind() {
	case syntax.Ident:
		return node.Tok().Text()
	case syntax.MemberExpr:
		return c.placeName(node.AsMember().Base().ID()) + "." + node.Tok().Text()
	case syntax.BracketExpr:
		return c.placeName(node.AsBracket().Base().ID()) + "[..]"
	case syntax.Paren:
		return c.placeName(node.AsParen().Inner().ID())
	}
	return "this expression"
}

// mirrors buildPlace's Ident case for an entity with no source node yet (a fresh `let` binding).
func (r *Result) identPlace(ent EntityID) Place {
	e := &r.Entities[ent]
	// a `move self` receiver is owned by the body, so it and its fields are mutable like `let mut`.
	mutable := e.Flags&EfMut != 0 || e.Flags&EfSelf != 0 && e.Flags&EfMove != 0
	deref := r.Types.Kind(e.Type) == KRef
	if deref {
		mutable = r.Types.Node(e.Type).Flags&flagMut != 0
	}
	return Place{Root: ent, Stable: !mutable && !deref, Mutable: mutable, Deref: deref}
}

// mirrors buildPlace's MemberExpr case for building base.field without a source node.
func (r *Result) fieldPlace(base Place, fld EntityID) Place {
	fe := &r.Entities[fld]
	p := Place{Root: base.Root, Fields: append(append([]EntityID(nil), base.Fields...), fld), Index: base.Index, Deref: base.Deref}
	p.Mutable = base.Mutable && fe.Flags&EfMut != 0
	p.Stable = base.Stable && fe.Flags&EfMut == 0
	return p
}

func (c *checker) rootOf(n syntax.NodeID) EntityID {
	p, ok := c.buildPlace(n)
	if !ok {
		return 0
	}
	return p.Root
}
