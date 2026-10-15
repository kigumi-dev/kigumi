package sem

// ReplacesSelf reports whether a call to m may leave another value in its `mut self` receiver; an interface requirement is answered for any implementation.
func (r *Result) ReplacesSelf(m EntityID) bool {
	if owner := r.Fn(m).Owner; owner != 0 && r.Entity(owner).Kind == EntInterface {
		return true
	}
	return r.replacesSelf(m)
}
