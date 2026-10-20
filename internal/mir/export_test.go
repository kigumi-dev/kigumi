package mir

import "kigumi/internal/sem"

func (p *Program) ReceiverMismatch(owner, got sem.TypeID) string {
	return p.receiverMismatch(owner, got)
}

func (p *Program) OpDescMismatch(f *Func, in Inst) string { return p.opDescMismatch(f, in) }

func (p *Program) IsrInertArgs(f *Func, in Inst, g *Func) []bool {
	return p.isrInertArgs(f, in, g, nil)
}
