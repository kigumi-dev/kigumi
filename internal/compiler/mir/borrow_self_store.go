package mir

// borrowsFrom reports whether b may alias root, directly or through a holder it borrows (`r` in `&r.src` for `r = &d`).
func (c *borrowCheck) borrowsFrom(b *borrowInfo, root LocalID) bool {
	seen := map[*borrowInfo]bool{}
	var walk func(b *borrowInfo) bool
	walk = func(b *borrowInfo) bool {
		if seen[b] {
			return false
		}
		seen[b] = true
		for _, o := range b.owners {
			if o.owner == root {
				return true
			}
			if h, ok := c.holders[o.owner]; ok && walk(h) {
				return true
			}
		}
		return false
	}
	return walk(b)
}
