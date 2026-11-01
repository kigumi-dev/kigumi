package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// copyable is what a consuming use duplicates instead of moving: Copy
// types, a type parameter included only when a `Copy` constraint proves it.
func (p *planner) copyable(t sem.TypeID) bool {
	switch p.R.Types.Kind(t) {
	case sem.KUntyped, sem.KVar, sem.KPrim:
		return true
	case sem.KRef:
		// A borrow is an alias of the caller's object: never duplicated,
		// never released.
		return false
	}
	return p.R.IsCopy(t)
}

// mustCopy: the scope still drops a named Copy local at exit, so an operand that consumes it gets a duplicate.
func (p *planner) mustCopy(f *midir.Func, in midir.Inst, a midir.LocalID) bool {
	l := f.Locals[a]
	if l.Ent == 0 || !p.copyable(l.Type) {
		return false
	}
	switch in.Op {
	case midir.OpDrop, midir.OpCopy, midir.OpBorrow, midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpIsVariant, midir.OpIsType, midir.OpPayload, midir.OpUnbox,
		midir.OpCellGet, midir.OpCellTake, midir.OpBinary, midir.OpUnary, midir.OpInterp, midir.OpBuiltin, midir.OpFutureNew, midir.OpFutureDone:
		return false
	}
	return true
}
