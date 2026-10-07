package sem

func (r *Result) DirectAdt(t TypeID) bool {
	return r.DirectAgg(t) && r.typeDecl(r.Types.Node(t).Ent).Form == FormAdt
}

func (r *Result) AggVariants(t TypeID) []EntityID {
	return r.typeDecl(r.Types.Node(t).Ent).Variants
}

func (r *Result) AggPayload(t TypeID, v EntityID) []TypeID {
	n := r.Types.Node(t)
	info := r.typeDecl(n.Ent)
	return r.payloadTypes(v, instSubst(info.Params, n.Args))
}
