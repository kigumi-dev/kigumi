package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

// explicitDrop lowers `v.drop()` to the drop scope exit uses, so the record and its fields go with it.
func (b *builder) explicitDrop(e *hir.Expr, recv LocalID, rest []LocalID) (LocalID, bool) {
	if e.Async || e.Witness != 0 || len(rest) != 0 || b.r.Entity(e.Ent).Kind != sem.EntFn {
		return 0, false
	}
	if owner := b.r.Fn(e.Ent).Owner; owner == 0 || b.r.TypeDecl(owner).Drop != e.Ent {
		return 0, false
	}
	b.emit(Inst{Op: OpDrop, Args: []LocalID{recv}, Node: e.Node})
	return b.emit(Inst{Op: OpUnit, Dst: b.temp(e.Type), Node: e.Node}), true
}
