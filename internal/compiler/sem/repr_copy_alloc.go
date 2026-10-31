package sem

func (r *Result) CopyAllocates(t TypeID) bool {
	if t == TyBytes {
		return true
	}
	t = r.underlyingRepr(t)
	switch n := r.Types.Node(t); n.Kind {
	case KParam:
		return true
	case KNamed:
		if n.Ent == r.langItem(0, 0, "FixedArray") {
			return true
		}
		switch r.typeDecl(n.Ent).Form {
		case FormRecord, FormAdt:
			return true
		}
	}
	return false
}
