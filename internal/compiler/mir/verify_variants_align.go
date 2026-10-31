package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

func (s *variantFacts) alignAliases(o *variantFacts) bool {
	type pair struct {
		l    midir.LocalID
		a, b vplace
	}
	var diff []pair
	for l, pl := range s.at {
		if b := o.where(l); b != pl {
			diff = append(diff, pair{l, pl, b})
		}
	}
	for l, pl := range o.at {
		if _, ok := s.at[l]; !ok && pl != s.where(l) {
			diff = append(diff, pair{l, s.where(l), pl})
		}
	}
	if len(diff) == 0 {
		return false
	}
	o.own()
	for _, d := range diff {
		s.rebase(d.a, d.l)
		o.rebase(d.b, d.l)
	}
	changed := false
	for _, d := range diff {
		if _, ok := s.at[d.l]; ok {
			changed = true
		}
		delete(s.at, d.l)
		delete(o.at, d.l)
	}
	return changed
}

func aliasable(pl vplace) bool { return pl.tag == tagValue || pl.tag == tagEmptied }

func (s *variantFacts) rebase(from vplace, l midir.LocalID) {
	own := vplace{root: l}
	if from == own {
		return
	}
	type fact struct {
		pl  vplace
		set variantSet
	}
	var moved []fact
	for pl, set := range s.known {
		if aliasable(pl) && pl.root == from.root && pl.path == from.path {
			moved = append(moved, fact{vplace{root: l, tag: pl.tag}, set})
		}
	}
	s.own()
	for _, m := range moved {
		s.setKnown(m.pl, m.set)
	}
	memo := map[*boolFact]*boolFact{}
	for c, b := range s.bools {
		nb, ok := memo[b]
		if !ok {
			nb = &boolFact{yes: b.yes.rebased(from, l), no: b.no.rebased(from, l)}
			if nb.yes == b.yes && nb.no == b.no {
				nb = b
			} else {
				s.roots[l] = true
			}
			memo[b] = nb
		}
		if nb != b {
			s.bools[c] = nb
		}
	}
}

func (im *implied) rebased(from vplace, l midir.LocalID) *implied {
	var out *implied
	for pl, set := range im.facts {
		if !aliasable(pl) || pl.root != from.root || pl.path != from.path {
			continue
		}
		if out == nil {
			out = &implied{facts: make(map[vplace]variantSet, len(im.facts)+1)}
			for k, v := range im.facts {
				out.facts[k] = v
			}
		}
		out.facts[vplace{root: l, tag: pl.tag}] = set
	}
	if out == nil {
		return im
	}
	return out
}
