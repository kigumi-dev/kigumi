package sem

// uninhabitedPayloads reports whether a constructor cannot be built because
// one of its fields has no values; a recursive type is conservatively
// inhabited.
func (c *checker) uninhabitedPayloads(fields []TypeID, seen map[TypeID]bool) bool {
	for _, f := range fields {
		if c.uninhabited(f, seen) {
			return true
		}
	}
	return false
}

// uninhabited reports whether t has no values: Never, an ADT whose every
// variant has an uninhabited payload, or a record with an uninhabited field.
func (c *checker) uninhabited(t TypeID, seen map[TypeID]bool) bool {
	tt := c.r.Types
	t = c.vars.resolve(t)
	if t == TyNever {
		return true
	}
	if seen[t] || len(seen) > 32 {
		return false
	}
	n := tt.Node(t)
	if n.Kind != KNamed {
		return false
	}
	info := c.r.typeDecl(n.Ent)
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	seen[t] = true
	defer delete(seen, t)
	switch info.Form {
	case FormAdt:
		if len(info.Variants) == 0 {
			return true
		}
		for _, v := range info.Variants {
			var fields []TypeID
			for _, p := range c.r.variant(v).Payload {
				fields = append(fields, tt.Subst(p, subst))
			}
			if !c.uninhabitedPayloads(fields, seen) {
				return false
			}
		}
		return true
	case FormRecord, FormResource:
		var fields []TypeID
		for _, f := range info.Fields {
			fields = append(fields, tt.Subst(c.r.Entities[f].Type, subst))
		}
		return c.uninhabitedPayloads(fields, seen)
	}
	return false
}
