package mir

import (
	"slices"

	"kigumi/internal/hir"
)

// loopHead also returns what the head bound in the loop scope, which each pass has to drop before the next binds.
func (b *builder) loopHead(head *hir.Expr) (LocalID, []LocalID, []guardMove) {
	before := len(b.scopes[len(b.scopes)-1].owned)
	c, moves := b.cond(head)
	return c, slices.Clone(b.scopes[len(b.scopes)-1].owned[before:]), moves
}

func (b *builder) dropLoopBound(bound []LocalID) {
	for i := len(bound) - 1; i >= 0; i-- {
		b.emit(Inst{Op: OpDrop, Args: []LocalID{bound[i]}, Node: b.dropNode(bound[i]), EndScope: true})
	}
}
