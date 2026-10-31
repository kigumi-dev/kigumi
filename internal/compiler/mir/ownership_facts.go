package mir

type ownershipFacts struct {
	*Program
	unowned map[*Func][]bool
}

func (p *ownershipFacts) unownedRefs(f *Func) []bool {
	if out, ok := p.unowned[f]; ok {
		return out
	}
	out := p.lentRefs(f)
	if p.unowned == nil {
		p.unowned = map[*Func][]bool{}
	}
	p.unowned[f] = out
	return out
}
