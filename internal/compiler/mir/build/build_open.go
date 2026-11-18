package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// openArgs fills the hidden locals of the parameters a matched type test opened from the box v: a runtime type,
// or a const parameter's value.
func (b *builder) openArgs(p *hir.Pat, v midir.LocalID) {
	r := b.r
	for i, a := range r.Types.Node(p.Target).Args {
		n := r.Types.Node(a)
		ent, ok := r.OpenedLocal(n.Ent)
		if n.Kind != sem.KParam || !ok {
			continue
		}
		dst := b.local(ent)
		op := "type.arg"
		if r.TypeParam(n.Ent).IsConst {
			op = "type.const"
		}
		b.emit(midir.Inst{Op: midir.OpBuiltin, Str: op, Dst: dst, Ent: p.Ent, Index: i, Args: []midir.LocalID{v}, Holes: []sem.EntityID{n.Ent}, Node: p.Node})
	}
}
