package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *Program) ReceiverMismatch(owner, got sem.TypeID) string {
	return newVerifier(p).receiverMismatch(owner, got)
}

func (p *Program) OpDescMismatch(f *midir.Func, in midir.Inst) string {
	return newVerifier(p).opDescMismatch(f, in)
}

func (p *Program) IsrInertArgs(f *midir.Func, in midir.Inst, g *midir.Func) []bool {
	return p.isrInertArgs(f, in, g, nil)
}

func ImmortalLocals(f *midir.Func, r *sem.Result) map[midir.LocalID]bool { return immortalLocals(f, r) }

func (p *Program) CountlessRefs(f *midir.Func) []bool { return p.countlessRefs(f) }
