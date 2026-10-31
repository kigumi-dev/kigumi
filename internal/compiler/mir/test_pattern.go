package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// testPattern yields a Bool local: does v match p.
func (b *builder) testPattern(p *hir.Pat, v midir.LocalID) midir.LocalID {
	switch p.Kind {
	case hir.PatWild, hir.PatBind:
		return b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(sem.TyBool), Lit: sem.Literal{Kind: sem.LitBool, Bool: true}, Type: sem.TyBool, Node: p.Node})
	case hir.PatLit:
		lit := b.expr(p.Lit)
		return b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(sem.TyBool), Str: "==", Args: []midir.LocalID{v, lit}, Node: p.Node})
	case hir.PatCtor:
		return b.testCtor(p, v)
	case hir.PatRecord:
		return b.testRecord(p, v)
	case hir.PatOr:
		cond := b.testPattern(p.Subs[0], v)
		for _, alt := range p.Subs[1:] {
			cond = b.orElse(cond, func() midir.LocalID { return b.testPattern(alt, v) })
		}
		return cond
	case hir.PatRange:
		return b.testRange(p, v)
	}
	return b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(sem.TyBool), Lit: sem.Literal{Kind: sem.LitBool}, Type: sem.TyBool, Node: p.Node})
}

// testRange yields lo <= v (< or <=) hi, `v`'s type throughout.
func (b *builder) testRange(p *hir.Pat, v midir.LocalID) midir.LocalID {
	lo := b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(sem.TyBool), Str: ">=", Args: []midir.LocalID{v, b.expr(p.Lo)}, Node: p.Node})
	hiOp := "<"
	if p.Inclusive {
		hiOp = "<="
	}
	return b.andThen(lo, func() midir.LocalID {
		return b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(sem.TyBool), Str: hiOp, Args: []midir.LocalID{v, b.expr(p.Hi)}, Node: p.Node})
	})
}

func (b *builder) testCtor(p *hir.Pat, v midir.LocalID) midir.LocalID {
	if p.IsType {
		cond := b.emit(midir.Inst{Op: midir.OpIsType, Dst: b.temp(sem.TyBool), Ent: p.Ent, Args: []midir.LocalID{v}, Node: p.Node})
		if len(p.Subs) == 1 {
			return b.andThen(cond, func() midir.LocalID {
				inner := b.emit(midir.Inst{Op: midir.OpUnbox, Dst: b.temp(p.Subs[0].Type), Args: []midir.LocalID{v}, Node: p.Node})
				return b.testPattern(p.Subs[0], inner)
			})
		}
		return cond
	}
	cond := b.emit(midir.Inst{Op: midir.OpIsVariant, Dst: b.temp(sem.TyBool), Ent: p.Ent, Args: []midir.LocalID{v}, Node: p.Node})
	for i, s := range p.Subs {
		if s.Kind == hir.PatWild || s.Kind == hir.PatBind {
			continue
		}
		cond = b.andThen(cond, func() midir.LocalID {
			payload := b.emit(midir.Inst{Op: midir.OpPayload, Dst: b.temp(s.Type), Args: []midir.LocalID{v}, Index: i, Node: s.Node})
			return b.testPattern(s, payload)
		})
	}
	return cond
}

func (b *builder) testRecord(p *hir.Pat, v midir.LocalID) midir.LocalID {
	var cond midir.LocalID
	if p.Variant {
		cond = b.emit(midir.Inst{Op: midir.OpIsVariant, Dst: b.temp(sem.TyBool), Ent: p.Ent, Args: []midir.LocalID{v}, Node: p.Node})
	} else {
		cond = b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(sem.TyBool), Lit: sem.Literal{Kind: sem.LitBool, Bool: true}, Type: sem.TyBool, Node: p.Node})
	}
	for _, f := range p.Fields {
		if f.Sub == nil || f.Sub.Kind == hir.PatWild || f.Sub.Kind == hir.PatBind {
			continue
		}
		cond = b.andThen(cond, func() midir.LocalID {
			return b.testPattern(f.Sub, b.fieldOf(f, v))
		})
	}
	return cond
}

// fieldOf reads the sub-value a record-pattern field names.
func (b *builder) fieldOf(f *hir.PatField, v midir.LocalID) midir.LocalID {
	if f.Payload {
		return b.emit(midir.Inst{Op: midir.OpPayload, Dst: b.temp(f.Type), Args: []midir.LocalID{v}, Index: f.Index, Node: f.Node})
	}
	return b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(f.Type), Args: []midir.LocalID{v}, Index: f.Index, Node: f.Node})
}
