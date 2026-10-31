package mir

import (
	"maps"

	midir "kigumi/internal/compiler/mir/ir"
)

type variantFacts struct {
	known map[vplace]variantSet
	at    map[midir.LocalID]vplace
	bools map[midir.LocalID]*boolFact
	roots []bool
	paths *pathIDs
	// shared says the maps are also another state's, so the first write copies them.
	shared bool
}

func newFacts(locals int) *variantFacts {
	return &variantFacts{roots: make([]bool, locals), paths: &pathIDs{ids: map[[2]uint32]uint32{}}}
}

// A clone shares the maps until either state writes; a state with no facts costs nothing to copy.
func (s *variantFacts) clone() *variantFacts {
	s.shared = len(s.known)+len(s.at)+len(s.bools) > 0
	return &variantFacts{roots: s.roots, paths: s.paths, known: s.known, at: s.at, bools: s.bools, shared: s.shared}
}

func (s *variantFacts) own() {
	if !s.shared {
		return
	}
	s.shared = false
	s.known, s.at, s.bools = maps.Clone(s.known), maps.Clone(s.at), maps.Clone(s.bools)
}

func (s *variantFacts) setKnown(pl vplace, set variantSet) {
	s.own()
	if s.known == nil {
		s.known = map[vplace]variantSet{}
	}
	s.known[pl] = set
	s.roots[pl.root] = true
}

func (s *variantFacts) setAt(l midir.LocalID, pl vplace) {
	s.own()
	if s.at == nil {
		s.at = map[midir.LocalID]vplace{}
	}
	s.at[l] = pl
	s.roots[pl.root] = true
}

func (s *variantFacts) setBool(l midir.LocalID, b *boolFact) {
	s.own()
	if s.bools == nil {
		s.bools = map[midir.LocalID]*boolFact{}
	}
	s.bools[l] = b
	s.roots[l] = true
	for _, im := range []*implied{b.yes, b.no} {
		for pl := range im.facts {
			s.roots[pl.root] = true
		}
	}
}

func (s *variantFacts) where(l midir.LocalID) vplace {
	if pl, ok := s.at[l]; ok {
		return pl
	}
	return vplace{root: l}
}

func (s *variantFacts) join(o *variantFacts) bool {
	s.own()
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

func (s *variantFacts) forget(l midir.LocalID) {
	if !s.roots[l] {
		return
	}
	s.own()
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

func (s *variantFacts) forgetBelow(root midir.LocalID) {
	var gone []vplace
	for pl := range s.known {
		if pl.root == root && pl.path != 0 {
			gone = append(gone, pl)
		}
	}
	s.drop(gone)
}

func (s *variantFacts) drop(places []vplace) {
	if len(places) == 0 {
		return
	}
	s.own()
	for _, pl := range places {
		delete(s.known, pl)
	}
}

// retire drops what a local that is never read again owns; facts about its place stay while a live alias still points there.
func (s *variantFacts) retire(l midir.LocalID) {
	if !s.roots[l] {
		return
	}
	_, aliased := s.at[l]
	_, boolean := s.bools[l]
	if aliased || boolean {
		s.own()
		delete(s.at, l)
		delete(s.bools, l)
	}
	for _, pl := range s.at {
		if pl.root == l {
			return
		}
	}
	var gone []vplace
	for pl := range s.known {
		if pl.about(l) {
			gone = append(gone, pl)
		}
	}
	s.drop(gone)
}

func (s *variantFacts) keepOnly(keep []bool) {
	var gone []midir.LocalID
	for c := range s.bools {
		if !keep[c] {
			gone = append(gone, c)
		}
	}
	if len(gone) == 0 {
		return
	}
	s.own()
	for _, c := range gone {
		delete(s.bools, c)
	}
}
