package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// takesScrutinee: the pattern moved a resource out of a named scrutinee and left only Copy parts,
// so emptying the scrutinee leaves the bindings as the only owners.
func (b *builder) takesScrutinee(p *hir.Pat, v LocalID) bool {
	if p == nil || p.Borrow || p.Kind == hir.PatBind || !b.wholeLocal(v) {
		return false
	}
	if p.Kind == hir.PatRecord && !p.Variant {
		return b.recordTaken(p)
	}
	moves, ok := b.patMovesAll(p)
	return ok && moves
}

// recordTaken: every field is bound whole by value and none is Copy, so no field of the record stays readable.
func (b *builder) recordTaken(p *hir.Pat) bool {
	for _, f := range p.Fields {
		if b.r.IsCopy(f.Type) || (f.Sub != nil && f.Sub.Kind != hir.PatBind) {
			return false
		}
	}
	moves, ok := b.patMovesAll(p)
	return ok && moves
}

// patMovesAll: moves when p binds a resource by value, ok when every part that is not Copy is bound by value.
func (b *builder) patMovesAll(p *hir.Pat) (moves, ok bool) {
	switch p.Kind {
	case hir.PatWild:
		return false, b.r.IsCopy(p.Type)
	case hir.PatBind:
		return b.bindMoves(p.Ent)
	case hir.PatLit, hir.PatRange, hir.PatNever:
		return false, true
	case hir.PatCtor:
		if p.IsType || p.Ent == 0 || b.r.Entity(p.Ent).Kind != sem.EntVariant || len(p.Subs) != len(b.r.Variant(p.Ent).Payload) {
			return false, false
		}
		ok = true
		for _, s := range p.Subs {
			m, k := b.patMovesAll(s)
			moves, ok = moves || m, ok && k
		}
		return moves, ok
	case hir.PatRecord:
		if len(p.Fields) != b.patFieldCount(p) {
			return false, false
		}
		ok = true
		for _, f := range p.Fields {
			var m, k bool
			if f.Sub != nil {
				m, k = b.patMovesAll(f.Sub)
			} else {
				m, k = b.bindMoves(f.Bind)
			}
			moves, ok = moves || m, ok && k
		}
		return moves, ok
	}
	return false, false
}

func (b *builder) bindMoves(ent sem.EntityID) (moves, ok bool) {
	bt := b.r.Entity(ent).Type
	if b.r.Types.Kind(bt) == sem.KRef {
		return false, false
	}
	return b.r.HoldsResource(bt), true
}

func (b *builder) patFieldCount(p *hir.Pat) int {
	if p.Ent == 0 {
		return -1
	}
	switch b.r.Entity(p.Ent).Kind {
	case sem.EntVariant:
		return len(b.r.Variant(p.Ent).Payload)
	case sem.EntType:
		return len(b.r.TypeDecl(p.Ent).Fields)
	}
	return -1
}

// takeScrutinee sits at the first moving binding, so a borrow conflict reports there only once.
func (b *builder) takeScrutinee(v LocalID, p *hir.Pat, guarded bool) {
	n := b.movingBind(p)
	t := b.emit(Inst{Op: OpMove, Dst: b.temp(b.fn.Locals[v].Type), Args: []LocalID{v}, Node: n, Guarded: guarded, Rest: true})
	b.emit(Inst{Op: OpDrop, Args: []LocalID{t}, Node: n, Rest: true})
}

func (b *builder) movingBind(p *hir.Pat) syntax.NodeID {
	switch p.Kind {
	case hir.PatBind:
		if m, _ := b.patMovesAll(p); m {
			return p.Node
		}
	case hir.PatCtor:
		for _, s := range p.Subs {
			if n := b.movingBind(s); n != 0 {
				return n
			}
		}
	case hir.PatRecord:
		for _, f := range p.Fields {
			if f.Sub != nil {
				if n := b.movingBind(f.Sub); n != 0 {
					return n
				}
			} else if m, _ := b.bindMoves(f.Bind); m {
				return f.Node
			}
		}
	}
	return 0
}

// takesField moves a bound resource field out of a named record, leaving the rest to drop with the record's scope.
func (b *builder) takesField(p *hir.Pat, f *hir.PatField, v LocalID) bool {
	if b.guardedArm || f.Payload || p.Variant || p.Borrow || !b.wholeLocal(v) || !b.r.HoldsResource(f.Type) || b.recordTaken(p) {
		return false
	}
	bind := f.Bind
	if f.Sub != nil {
		if f.Sub.Kind != hir.PatBind {
			return false
		}
		bind = f.Sub.Ent
	}
	return b.r.Types.Kind(b.r.Entity(bind).Type) != sem.KRef
}
