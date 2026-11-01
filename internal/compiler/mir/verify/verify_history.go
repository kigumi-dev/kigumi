package verify

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

type verifiedFlow struct {
	hash uint64
	weak []bool
}

type verificationHistory map[*midir.Func]verifiedFlow

// Session verifies one program round after round; a body that passed the flow rules and has not changed skips them.
type Session struct{ verified verificationHistory }

func (s *Session) Run(p *mir.Program) error {
	if s.verified == nil {
		s.verified = verificationHistory{}
	}
	return newVerifier(p, s.verified).check()
}

func (p *verifier) rememberFlow(f *midir.Func, hash uint64) {
	if p.verified == nil {
		p.verified = verificationHistory{}
	}
	p.verified[f] = verifiedFlow{hash: hash, weak: p.body(f).weakDefs}
}

func (p *verifier) pruneHistory(fs []*midir.Func) {
	if len(p.verified) == 0 {
		return
	}
	live := make(map[*midir.Func]bool, len(fs))
	for _, f := range fs {
		live[f] = true
	}
	for f := range p.verified {
		if !live[f] {
			delete(p.verified, f)
		}
	}
}
