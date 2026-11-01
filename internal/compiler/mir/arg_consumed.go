package mir

import midir "kigumi/internal/compiler/mir/ir"

func (p *Program) ArgConsumed(f *midir.Func, in midir.Inst, i int) bool { return p.Consumes(f, in, i) }

// Consumes reports whether argument i is stored, moved, boxed or handed to
// an owning callee; reads, arithmetic, display and borrowed receivers do not.
func (p *Program) Consumes(f *midir.Func, in midir.Inst, i int) bool {
	if p.InPlace(in, i) || p.LentParam(in, i) || p.LentByType(f, &in, i) || p.LentStore(f, in, i) {
		return false
	}
	switch in.Op {
	case midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpIsVariant, midir.OpIsType, midir.OpPayload, midir.OpUnbox,
		midir.OpCellGet, midir.OpCellTake, midir.OpBinary, midir.OpUnary, midir.OpInterp, midir.OpBuiltin, midir.OpBorrow, midir.OpLendBox, midir.OpFutureNew, midir.OpFutureDone, midir.OpShare, midir.OpClosure, midir.OpBind:
		return false
	}
	return true
}
