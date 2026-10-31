package sem

// deriveBlocker names the first component of id's declaration that keeps iface from being derived.
func (r *Result) deriveBlocker(id EntityID, iface string) string {
	info := r.typeDecl(id)
	if _, own := info.Members[deriveMethods[iface]]; own {
		return ""
	}
	g := &deriver{r: r}
	ok := func(t TypeID) bool {
		if isProto(iface) {
			return r.deriveOf(t, iface)
		}
		return g.derivable(t, iface)
	}
	for _, fld := range info.Fields {
		if t := r.field(fld).Type; !ok(t) {
			return "its field `" + r.Entities[fld].Name + "` of type `" + r.TypeString(t) + "` has no `" + iface + "`"
		}
	}
	for _, v := range info.Variants {
		for _, t := range r.variant(v).Payload {
			if !ok(t) {
				return "a payload of `" + r.Entities[v].Name + "` of type `" + r.TypeString(t) + "` has no `" + iface + "`"
			}
		}
	}
	switch info.Form {
	case FormRecord, FormAdt:
	case FormResource:
		if isProto(iface) {
			return "a resource has no structural `" + iface + "`"
		}
	default:
		return "`" + iface + "` is derived only for a record or an enum"
	}
	return ""
}
