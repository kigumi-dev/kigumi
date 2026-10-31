package mir

import "kigumi/internal/compiler/sem"

func (p *Program) ReceiverMismatch(owner, got sem.TypeID) string {
	return newVerifier(p).receiverMismatch(owner, got)
}

func (p *Program) OpDescMismatch(f *Func, in Inst) string {
	return newVerifier(p).opDescMismatch(f, in)
}

func (p *Program) IsrInertArgs(f *Func, in Inst, g *Func) []bool {
	return p.isrInertArgs(f, in, g, nil)
}

func ImmortalLocals(f *Func, r *sem.Result) map[LocalID]bool { return immortalLocals(f, r) }

func (p *Program) CountlessRefs(f *Func) []bool { return p.countlessRefs(f) }
