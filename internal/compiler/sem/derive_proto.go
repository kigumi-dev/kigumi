package sem

func (r *Result) derivable(t TypeID, proto string) bool {
	n := r.Types.Node(t)
	if n.Kind != KNamed {
		return false
	}
	if form := r.typeDecl(n.Ent).Form; form != FormRecord && form != FormAdt {
		return false
	}
	return r.deriveOf(t, proto)
}

// protocolMismatch explains why t's own method fails the requirement, or
// "" if there is no such method.
func (r *Result) protocolMismatch(t TypeID, proto string) string {
	n := r.Types.Node(t)
	iface := r.langItem(0, 0, proto)
	if n.Kind != KNamed || iface == 0 {
		return ""
	}
	set, ok := r.typeDecl(n.Ent).Members[deriveMethods[proto]]
	if !ok || len(r.Overloads[set].Members) == 0 {
		return ""
	}
	req := r.requirement(iface, deriveMethods[proto])
	if req == 0 {
		return ""
	}
	_, why, _, _ := r.findWitness(t, req, map[EntityID]TypeID{r.iface(iface).SelfParam: t}, r.Entities[n.Ent].Pkg)
	return why
}
