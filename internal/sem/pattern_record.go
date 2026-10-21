package sem

import "kigumi/internal/syntax"

// recordPattern handles `T { a, b: p }` over a record or a variant with
// named payload.
func (c *checker) recordPattern(p syntax.NodeID, scrutinee TypeID, seen map[string]syntax.NodeID) PatInfo {
	rec := c.t.View(p).AsPatRecord()
	tt := c.r.Types
	fields := rec.Fields().AppendIDs(nil)
	ent, ok := c.resolveCtorPath(p, rec.Path().ID(), scrutinee)
	if !ok || ent == 0 {
		c.skipFields(fields, seen)
		c.setType(p, TyPoison)
		return PatInfo{Kind: PatWild, Type: TyPoison}
	}
	c.info.Uses[p] = ent
	e := &c.r.Entities[ent]
	sn := tt.Node(scrutinee)
	subst := map[EntityID]TypeID{}
	var info PatInfo
	var lookup func(name string) (TypeID, EntityID, bool)
	switch {
	case e.Kind == EntType && c.r.typeDecl(ent).Form != FormAdt && c.r.typeDecl(ent).Form != FormDefined:
		if scrutinee != TyPoison && (sn.Kind != KNamed || sn.Ent != ent) {
			c.errAt(p, cPatternKind, scrutinee)
			c.skipFields(fields, seen)
			c.setType(p, TyPoison)
			return PatInfo{Kind: PatWild, Type: scrutinee}
		}
		for i, prm := range c.r.typeDecl(ent).Params {
			if i < len(sn.Args) {
				subst[prm] = sn.Args[i]
			}
		}
		info = PatInfo{Kind: PatRecordTy, Type: scrutinee}
		lookup = func(name string) (TypeID, EntityID, bool) {
			fld := c.r.findField(ent, name)
			if fld == 0 {
				return 0, 0, false
			}
			return tt.Subst(c.r.Entities[fld].Type, subst), fld, true
		}
	case e.Kind == EntVariant:
		if e.Flags&EfNamedPayload == 0 {
			c.errAt(p, cPatternRecordPositional, e.Name, e.Name)
			c.skipFields(fields, seen)
			c.setType(p, TyPoison)
			return PatInfo{Kind: PatVariant, Variant: ent, Type: scrutinee}
		}
		if scrutinee != TyPoison && (sn.Kind != KNamed || sn.Ent != e.Parent) {
			c.errAt(p, cPatternWrongType, e.Name, scrutinee)
			c.skipFields(fields, seen)
			c.setType(p, TyPoison)
			return PatInfo{Kind: PatVariant, Variant: ent, Type: scrutinee}
		}
		for i, prm := range c.r.typeDecl(e.Parent).Params {
			if i < len(sn.Args) {
				subst[prm] = sn.Args[i]
			}
		}
		vi := c.r.variant(ent)
		info = PatInfo{Kind: PatVariant, Variant: ent, Type: scrutinee}
		lookup = func(name string) (TypeID, EntityID, bool) {
			for i, n := range vi.Names {
				if n == name {
					return tt.Subst(vi.Payload[i], subst), 0, true
				}
			}
			return 0, 0, false
		}
	default:
		c.errAt(p, cPatternKind, scrutinee)
		c.skipFields(fields, seen)
		c.setType(p, TyPoison)
		return PatInfo{Kind: PatWild, Type: scrutinee}
	}
	used := map[string]bool{}
	for _, f := range fields {
		fn := c.t.View(f).AsPatField()
		name := fn.Name().Text()
		ft, fld, ok := lookup(name)
		if !ok {
			c.errAt(f, cPatternUnknownField, e.Name, name)
			c.info.Types[p] = TyPoison
			if fn.Sub().Valid() {
				c.patternInner(fn.Sub().ID(), TyPoison, false, seen)
			}
			continue
		}
		if used[name] {
			c.errAt(f, cPatternDuplicateField, name)
		}
		used[name] = true
		if fld != 0 {
			c.r.useEntity(c.f, f, fld)
			c.info.Uses[f] = fld
			info.Fields = append(info.Fields, fld)
		}
		c.setType(f, ft)
		if fn.Sub().Valid() {
			c.patternInField(fld, func() {
				if c.patternInner(fn.Sub().ID(), ft, false, seen).Moves {
					info.Moves = true
				}
			})
			continue
		}
		if prev, dup := seen[name]; dup && prev != f {
			c.errAt(f, cPatternDuplicateBinding, name)
		}
		seen[name] = f
		c.noteCopyBinding(c.declareLocal(f, name, c.bindingType(ft), false), fld)
		c.patternInField(fld, func() { c.notePatternMove(ft) })
		if !c.patBorrow && !c.r.isCopy(ft) {
			info.Moves = true
		}
	}
	return info
}

func (c *checker) skipFields(fields []syntax.NodeID, seen map[string]syntax.NodeID) {
	for _, f := range fields {
		if sub := c.t.View(f).AsPatField().Sub().ID(); sub != 0 {
			c.patternInner(sub, TyPoison, false, seen)
		}
	}
}

// synthIs types `e is Pattern`: a Bool whose pattern bindings are
// visible to the rest of the enclosing condition and then-block.
func (c *checker) synthIs(n syntax.NodeID) TypeID {
	is := c.t.View(n).AsIs()
	subject, pat := is.Left().ID(), is.Pattern().ID()
	scrutinee := c.vars.resolve(c.synth(subject))
	if c.r.Types.Kind(scrutinee) == KUntyped {
		scrutinee = c.adopt(c.literalNode(subject), defaultOf(scrutinee))
	}
	rp := c.restrictedAliasOf(subject)
	if c.inCond == 0 {
		saved := c.scope
		c.pushScope(ScopeArm, n)
		c.checkPatternOn(pat, subject, scrutinee, false, c.scope)
		c.taintPatternBindings(pat, rp)
		if len(c.r.Scopes[c.scope].Names) > 0 {
			c.errAt(n, cIsBindingOutsideCond)
		}
		c.popScope(saved)
	} else if c.negated {
		saved := c.scope
		c.pushScope(ScopeArm, n)
		c.checkPatternOn(pat, subject, scrutinee, false, c.scope)
		c.taintPatternBindings(pat, rp)
		c.popScope(saved)
	} else {
		c.checkPatternOn(pat, subject, scrutinee, false, c.scope)
		c.taintPatternBindings(pat, rp)
	}
	c.info.Narrow[n] = c.snapshotFacts()
	c.info.Matches[n] = MatchInfo{}
	return TyBool
}
