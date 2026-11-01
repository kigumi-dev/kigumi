package build

import (
	"slices"

	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
)

// loopHead also returns what the head bound in the loop scope, which each pass has to drop before the next binds.
func (b *builder) loopHead(head *hir.Expr) (midir.LocalID, []midir.LocalID, []guardMove) {
	before := len(b.scopes[len(b.scopes)-1].owned)
	c, moves := b.cond(head)
	return c, slices.Clone(b.scopes[len(b.scopes)-1].owned[before:]), moves
}

func (b *builder) dropLoopBound(bound []midir.LocalID) {
	for i := len(bound) - 1; i >= 0; i-- {
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{bound[i]}, Node: b.dropNode(bound[i]), EndScope: true})
	}
}
