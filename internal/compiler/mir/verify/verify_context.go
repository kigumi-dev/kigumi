package verify

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

type verifier struct {
	ownershipFacts
	bodies   map[*midir.Func]*verifiedBody
	verified verificationHistory
}

type verifiedBody struct {
	weakDefs []bool
	live     *edgeLive
	yielders map[midir.LocalID][]midir.Inst
}

// Program checks every function of p.
func Program(p *mir.Program) error { return new(Session).Run(p) }

func newVerifier(p *mir.Program, history verificationHistory) *verifier {
	return &verifier{ownershipFacts: ownershipFacts{Program: p}, verified: history}
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
