package sem

// InPlaceCopy reports a Copy record built only from scalars and such records: a slot already holding one takes another's fields without a new box.
func (r *Result) InPlaceCopy(t TypeID) bool {
	tt := r.Types
	if t == 0 || tt.Kind(t) != KNamed || r.typeDecl(tt.Node(t).Ent).Form != FormRecord || !r.DirectAgg(t) || !r.isCopy(t) {
		return false
	}
	for _, f := range r.AggFields(t) {
		if f != TyBool && f != TyChar && !tt.IsNumeric(f) && !r.InPlaceCopy(f) {
			return false
		}
	}
	return true
}
