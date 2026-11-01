package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// asm lowers an asm block to one OpAsm over its input values (in and inout
// operands in order); the backend reads the rest from sem.AsmInfo.
func (b *builder) asm(e *hir.Expr) midir.LocalID {
	var args []midir.LocalID
	for _, a := range e.Args {
		args = append(args, b.expr(a))
	}
	dst := b.emit(midir.Inst{Op: midir.OpAsm, Dst: b.temp(e.Type), Type: e.Type, Args: args, Node: e.Node})
	if e.Type == sem.TyNever {
		b.term(midir.Term{Op: midir.TermUnreachable})
		return b.never(e.Node)
	}
	return dst
}
