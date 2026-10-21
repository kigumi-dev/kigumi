package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// sem checks a Void block's last non-Void expression as a discarded statement, so it is no tail value.
func (l *lowerer) discardedTail(block sem.TypeID, stmt syntax.NodeID) bool {
	t := l.typeOf(stmt)
	return block == sem.TyUnit && t != sem.TyUnit && t != sem.TyNever && t != sem.TyPoison && t != 0
}
