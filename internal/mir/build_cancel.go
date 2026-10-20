package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// cancelEdge unwinds like an early return, so failure-only cleanups stay skipped.
func (b *builder) cancelEdge(n syntax.NodeID) {
	cancelled := b.emit(Inst{Op: OpCancelled, Dst: b.temp(sem.TyBool), Node: n})
	cancel, cont := b.newBlockAt(), b.newBlockAt()
	b.term(Term{Op: TermBranch, Args: []LocalID{cancelled}, Targets: []BlockID{cancel, cont}})
	b.cur = cancel
	b.unwindTo(0, false)
	b.emit(Inst{Op: OpCancelDone, Node: n})
	b.term(Term{Op: TermUnreachable})
	b.cur = cont
}
