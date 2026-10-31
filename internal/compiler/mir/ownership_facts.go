package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

type ownershipFacts struct {
	*Program
	unowned map[*midir.Func][]bool
}

func (p *ownershipFacts) unownedRefs(f *midir.Func) []bool {
	if out, ok := p.unowned[f]; ok {
		return out
	}
	out := ownquery.LentRefs(p.R, f)
	if p.unowned == nil {
		p.unowned = map[*midir.Func][]bool{}
	}
	p.unowned[f] = out
	return out
}
