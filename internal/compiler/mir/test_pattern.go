package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
)

// testPattern yields a Bool local: does v match p.
func (b *builder) testPattern(p *hir.Pat, v LocalID) LocalID {
	switch p.Kind {
	case hir.PatWild, hir.PatBind:
		return b.emit(Inst{Op: OpConst, Dst: b.temp(sem.TyBool), Lit: sem.Literal{Kind: sem.LitBool, Bool: true}, Type: sem.TyBool, Node: p.Node})
	case hir.PatLit:
		lit := b.expr(p.Lit)
		return b.emit(Inst{Op: OpBinary, Dst: b.temp(sem.TyBool), Str: "==", Args: []LocalID{v, lit}, Node: p.Node})
	case hir.PatCtor:
		return b.testCtor(p, v)
	case hir.PatRecord:
		return b.testRecord(p, v)
	case hir.PatOr:
		cond := b.testPattern(p.Subs[0], v)
		for _, alt := range p.Subs[1:] {
			cond = b.orElse(cond, func() LocalID { return b.testPattern(alt, v) })
		}
		return cond
	case hir.PatRange:
		return b.testRange(p, v)
	}
	return b.emit(Inst{Op: OpConst, Dst: b.temp(sem.TyBool), Lit: sem.Literal{Kind: sem.LitBool}, Type: sem.TyBool, Node: p.Node})
}

// testRange yields lo <= v (< or <=) hi, `v`'s type throughout.
func (b *builder) testRange(p *hir.Pat, v LocalID) LocalID {
	lo := b.emit(Inst{Op: OpBinary, Dst: b.temp(sem.TyBool), Str: ">=", Args: []LocalID{v, b.expr(p.Lo)}, Node: p.Node})
	hiOp := "<"
	if p.Inclusive {
		hiOp = "<="
	}
	return b.andThen(lo, func() LocalID {
		return b.emit(Inst{Op: OpBinary, Dst: b.temp(sem.TyBool), Str: hiOp, Args: []LocalID{v, b.expr(p.Hi)}, Node: p.Node})
	})
}

func (b *builder) testCtor(p *hir.Pat, v LocalID) LocalID {
	if p.IsType {
		cond := b.emit(Inst{Op: OpIsType, Dst: b.temp(sem.TyBool), Ent: p.Ent, Args: []LocalID{v}, Node: p.Node})
		if len(p.Subs) == 1 {
			return b.andThen(cond, func() LocalID {
				inner := b.emit(Inst{Op: OpUnbox, Dst: b.temp(p.Subs[0].Type), Args: []LocalID{v}, Node: p.Node})
				return b.testPattern(p.Subs[0], inner)
			})
		}
		return cond
	}
	cond := b.emit(Inst{Op: OpIsVariant, Dst: b.temp(sem.TyBool), Ent: p.Ent, Args: []LocalID{v}, Node: p.Node})
	for i, s := range p.Subs {
		if s.Kind == hir.PatWild || s.Kind == hir.PatBind {
			continue
		}
		cond = b.andThen(cond, func() LocalID {
			payload := b.emit(Inst{Op: OpPayload, Dst: b.temp(s.Type), Args: []LocalID{v}, Index: i, Node: s.Node})
			return b.testPattern(s, payload)
		})
	}
	return cond
}

func (b *builder) testRecord(p *hir.Pat, v LocalID) LocalID {
	var cond LocalID
	if p.Variant {
		cond = b.emit(Inst{Op: OpIsVariant, Dst: b.temp(sem.TyBool), Ent: p.Ent, Args: []LocalID{v}, Node: p.Node})
	} else {
		cond = b.emit(Inst{Op: OpConst, Dst: b.temp(sem.TyBool), Lit: sem.Literal{Kind: sem.LitBool, Bool: true}, Type: sem.TyBool, Node: p.Node})
	}
	for _, f := range p.Fields {
		if f.Sub == nil || f.Sub.Kind == hir.PatWild || f.Sub.Kind == hir.PatBind {
			continue
		}
		cond = b.andThen(cond, func() LocalID {
			return b.testPattern(f.Sub, b.fieldOf(f, v))
		})
	}
	return cond
}

// fieldOf reads the sub-value a record-pattern field names.
func (b *builder) fieldOf(f *hir.PatField, v LocalID) LocalID {
	if f.Payload {
		return b.emit(Inst{Op: OpPayload, Dst: b.temp(f.Type), Args: []LocalID{v}, Index: f.Index, Node: f.Node})
	}
	return b.emit(Inst{Op: OpField, Dst: b.temp(f.Type), Args: []LocalID{v}, Index: f.Index, Node: f.Node})
}
