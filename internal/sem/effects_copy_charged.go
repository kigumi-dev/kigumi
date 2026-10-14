package sem

func (r *Result) CopyCharged(s CopySite) bool {
	for _, e := range r.edgesOf(s.Owner) {
		if e.Kind == EdgeAlloc && copyWhy(e.Why) && e.File == s.File && e.Node == s.Node {
			return true
		}
	}
	return false
}
