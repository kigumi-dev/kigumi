package mir

func (s *variantFacts) fold() {
	if len(s.bools) == 0 {
		return
	}
	memo := map[*boolFact]*boolFact{}
	for c, b := range s.bools {
		nb, ok := memo[b]
		if !ok {
			nb = &boolFact{yes: s.foldSide(b.yes), no: s.foldSide(b.no)}
			if sameBool(nb, b) {
				nb = b
			}
			memo[b] = nb
		}
		s.bools[c] = nb
	}
}

func (s *variantFacts) foldSide(im *implied) *implied {
	if im.never || len(s.known) == 0 {
		return im
	}
	if s.covers(im) {
		return im
	}
	facts := make(map[vplace]variantSet, len(im.facts)+len(s.known))
	for pl, set := range im.facts {
		if cur, ok := s.known[pl]; ok {
			if set = meet(cur, set); len(set.list) == 0 {
				return neverImplied
			}
		}
		facts[pl] = set
	}
	for pl, set := range s.known {
		if _, ok := facts[pl]; !ok {
			facts[pl] = set
		}
	}
	return &implied{facts: facts}
}

func (s *variantFacts) assume(c LocalID, value bool) bool {
	b := s.bools[c]
	if b == nil {
		return true
	}
	im := b.yes
	if !value {
		im = b.no
	}
	if im.never {
		return false
	}
	for pl, set := range im.facts {
		if cur, ok := s.known[pl]; ok {
			set = meet(cur, set)
		}
		switch {
		case len(set.list) == 0:
			return false
		case set.full():
			delete(s.known, pl)
		default:
			s.setKnown(pl, set)
		}
	}
	nb := &boolFact{yes: neverImplied, no: im}
	if value {
		nb = &boolFact{yes: im, no: neverImplied}
	}
	s.markFacts(nb)
	for l, x := range s.bools {
		if x == b {
			s.bools[l] = nb
		}
	}
	return true
}

func (s *variantFacts) markFacts(b *boolFact) {
	for _, im := range []*implied{b.yes, b.no} {
		for pl := range im.facts {
			s.roots[pl.root] = true
		}
	}
}

func (s *variantFacts) covers(im *implied) bool {
	for pl, set := range s.known {
		cur, ok := im.facts[pl]
		if !ok || !cur.equal(set) {
			return false
		}
	}
	for pl, set := range im.facts {
		if cur, ok := s.known[pl]; ok && !cur.equal(set) {
			return false
		}
	}
	return true
}
