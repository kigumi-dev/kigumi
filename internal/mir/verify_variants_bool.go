package mir

type implied struct {
	never bool
	facts map[vplace]variantSet
}

var neverImplied = &implied{never: true}

// boolFact says what follows from each value of a boolean local. It is never
// edited once made, so locals that copy one another share it.
type boolFact struct {
	yes, no *implied
}

func (b *boolFact) mentions(l LocalID) bool {
	for _, im := range []*implied{b.yes, b.no} {
		for pl := range im.facts {
			if pl.about(l) {
				return true
			}
		}
	}
	return false
}

func (im *implied) without(l LocalID) *implied {
	out := &implied{never: im.never, facts: map[vplace]variantSet{}}
	for pl, set := range im.facts {
		if !pl.about(l) {
			out.facts[pl] = set
		}
	}
	return out
}

func (b *boolFact) without(l LocalID) *boolFact {
	return &boolFact{yes: b.yes.without(l), no: b.no.without(l)}
}

func joinImplied(a, b *implied) *implied {
	switch {
	case a.never:
		return b
	case b.never:
		return a
	}
	out := &implied{facts: map[vplace]variantSet{}}
	for pl, sa := range a.facts {
		if sb, ok := b.facts[pl]; ok {
			if j := joinSets(sa, sb); !j.full() {
				out.facts[pl] = j
			}
		}
	}
	return out
}

func joinBool(a, b *boolFact, memo map[[2]*boolFact]*boolFact) *boolFact {
	if b == nil {
		return nil
	}
	if a == b {
		return a
	}
	key := [2]*boolFact{a, b}
	if j, ok := memo[key]; ok {
		return j
	}
	j := &boolFact{yes: joinImplied(a.yes, b.yes), no: joinImplied(a.no, b.no)}
	memo[key] = j
	return j
}

func sameImplied(a, b *implied) bool {
	if a.never != b.never || len(a.facts) != len(b.facts) {
		return false
	}
	for pl, s := range a.facts {
		if !s.equal(b.facts[pl]) {
			return false
		}
	}
	return true
}

func sameBool(a, b *boolFact) bool {
	return a == b || (sameImplied(a.yes, b.yes) && sameImplied(a.no, b.no))
}
