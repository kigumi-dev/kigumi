package mir

type verifier struct {
	ownershipFacts
	bodies map[*Func]*verifiedBody
}

type verifiedBody struct {
	weakDefs []bool
	live     *edgeLive
	yielders map[LocalID][]Inst
}

func (p *Program) Verify() error {
	return newVerifier(p).check()
}

func newVerifier(p *Program) *verifier {
	return &verifier{ownershipFacts: ownershipFacts{Program: p}}
}

func (p *verifier) body(f *Func) *verifiedBody {
	if body := p.bodies[f]; body != nil {
		return body
	}
	body := &verifiedBody{}
	if p.bodies == nil {
		p.bodies = map[*Func]*verifiedBody{}
	}
	p.bodies[f] = body
	return body
}
