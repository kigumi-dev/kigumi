package verify

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func ReceiverMismatch(p *mir.Program, owner, got sem.TypeID) string {
	return newVerifier(p, nil).receiverMismatch(owner, got)
}

func OpDescMismatch(p *mir.Program, f *midir.Func, in midir.Inst) string {
	return newVerifier(p, nil).opDescMismatch(f, in)
}

func (s *Session) Flow(f *midir.Func) (hash uint64, weak []bool, ok bool) {
	v, ok := s.verified[f]
	return v.hash, v.weak, ok
}

func (s *Session) SetFlow(f *midir.Func, hash uint64, weak []bool) {
	s.verified[f] = verifiedFlow{hash: hash, weak: weak}
}

func (s *Session) Remembered() int { return len(s.verified) }
