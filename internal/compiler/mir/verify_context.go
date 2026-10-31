package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

type verifier struct {
	ownershipFacts
	bodies map[*midir.Func]*verifiedBody
}

type verifiedBody struct {
	weakDefs []bool
	live     *edgeLive
	yielders map[midir.LocalID][]midir.Inst
}

func (p *Program) Verify() error {
	return newVerifier(p).check()
}

func newVerifier(p *Program) *verifier {
	return &verifier{ownershipFacts: ownershipFacts{Program: p}}
}

func (p *verifier) body(f *midir.Func) *verifiedBody {
	if body := p.bodies[f]; body != nil {
		return body
	}
	body := &verifiedBody{}
	if p.bodies == nil {
		p.bodies = map[*midir.Func]*verifiedBody{}
	}
	p.bodies[f] = body
	return body
}
