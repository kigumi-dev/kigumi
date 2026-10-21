package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// sem checks a Unit block's last non-Unit expression as a discarded statement, so it is no tail value.
func (l *lowerer) discardedTail(block sem.TypeID, stmt syntax.NodeID) bool {
	t := l.typeOf(stmt)
	return block == sem.TyUnit && t != sem.TyUnit && t != sem.TyNever && t != sem.TyPoison && t != 0
}
