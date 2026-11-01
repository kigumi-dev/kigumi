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
