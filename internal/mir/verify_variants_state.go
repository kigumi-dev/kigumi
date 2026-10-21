package mir

type variantFacts struct {
	known map[vplace]variantSet
	at    map[LocalID]vplace
	bools map[LocalID]*boolFact
	roots []bool
	paths *pathIDs
}

func newFacts(locals int) *variantFacts {
	return &variantFacts{roots: make([]bool, locals), paths: &pathIDs{ids: map[[2]uint32]uint32{}}}
}

// The maps stay nil until something is learned, so a state with no facts costs nothing to copy.
func (s *variantFacts) clone() *variantFacts {
	c := &variantFacts{roots: s.roots, paths: s.paths}
	if len(s.known) > 0 {
		c.known = make(map[vplace]variantSet, len(s.known))
		for k, v := range s.known {
			c.known[k] = v
		}
	}
	if len(s.at) > 0 {
		c.at = make(map[LocalID]vplace, len(s.at))
		for k, v := range s.at {
			c.at[k] = v
		}
	}
	if len(s.bools) > 0 {
		c.bools = make(map[LocalID]*boolFact, len(s.bools))
		for k, v := range s.bools {
			c.bools[k] = v
		}
	}
	return c
}

func (s *variantFacts) setKnown(pl vplace, set variantSet) {
	if s.known == nil {
		s.known = map[vplace]variantSet{}
	}
	s.known[pl] = set
	s.roots[pl.root] = true
}

func (s *variantFacts) setAt(l LocalID, pl vplace) {
	if s.at == nil {
		s.at = map[LocalID]vplace{}
	}
	s.at[l] = pl
	s.roots[pl.root] = true
}

func (s *variantFacts) setBool(l LocalID, b *boolFact) {
	if s.bools == nil {
		s.bools = map[LocalID]*boolFact{}
	}
	s.bools[l] = b
	s.roots[l] = true
	for _, im := range []*implied{b.yes, b.no} {
		for pl := range im.facts {
			s.roots[pl.root] = true
		}
	}
}

func (s *variantFacts) where(l LocalID) vplace {
	if pl, ok := s.at[l]; ok {
		return pl
	}
	return vplace{root: l}
}

func (s *variantFacts) join(o *variantFacts) bool {
	changed := s.alignAliases(o)
	for pl, set := range s.known {
		os, ok := o.known[pl]
		switch {
		case !ok:
			delete(s.known, pl)
			changed = true
		default:
			j := joinSets(set, os)
			switch {
			case j.full():
				delete(s.known, pl)
				changed = true
			case !j.equal(set):
				s.known[pl], changed = j, true
			}
		}
	}
	memo := map[[2]*boolFact]*boolFact{}
	for c, b := range s.bools {
		j := joinBool(b, o.bools[c], memo)
		switch {
		case j == nil:
			delete(s.bools, c)
			changed = true
		case !sameBool(j, b):
			s.bools[c], changed = j, true
		}
	}
	return changed
}

func (s *variantFacts) forget(l LocalID) {
	if !s.roots[l] {
		return
	}
	delete(s.at, l)
	for m, pl := range s.at {
		if pl.root == l {
			delete(s.at, m)
		}
	}
	for pl := range s.known {
		if pl.about(l) {
			delete(s.known, pl)
		}
	}
	delete(s.bools, l)
	memo := map[*boolFact]*boolFact{}
	for c, b := range s.bools {
		if !b.mentions(l) {
			continue
		}
		if _, ok := memo[b]; !ok {
			memo[b] = b.without(l)
		}
		s.bools[c] = memo[b]
	}
}

func (s *variantFacts) forgetBelow(root LocalID) {
	for pl := range s.known {
		if pl.root == root && pl.path != 0 {
			delete(s.known, pl)
		}
	}
}

func (s *variantFacts) keepOnly(keep []bool) {
	for c := range s.bools {
		if !keep[c] {
			delete(s.bools, c)
		}
	}
}
