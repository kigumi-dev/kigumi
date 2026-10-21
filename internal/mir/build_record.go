package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

func (b *builder) optField(e *hir.Expr) LocalID {
	n, t := e.Node, e.Type
	inner := b.r.Types.Node(t).Args[0]
	var base LocalID
	if e.Ent != 0 {
		base = b.methodReceiver(e, inner)
	} else {
		base = b.expr(e.Args[0])
	}
	some := b.variantNamed(b.r.Types.OptionEnt(), "Some")
	isSome := b.emit(Inst{Op: OpIsVariant, Dst: b.temp(sem.TyBool), Ent: some, Args: []LocalID{base}, Node: n})
	result := b.temp(t)
	yes, no, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(Term{Op: TermBranch, Args: []LocalID{isSome}, Targets: []BlockID{yes, no}})
	b.cur = yes
	payload := b.emit(Inst{Op: OpPayload, Dst: b.temp(b.r.Types.Node(e.Args[0].Type).Args[0]), Args: []LocalID{base}, Node: n})
	var val LocalID
	if e.Ent != 0 {
		val = b.methodValue(e, payload, inner)
	} else {
		val = b.emit(Inst{Op: OpField, Dst: b.temp(inner), Args: []LocalID{payload}, Index: e.Index, Node: n})
	}
	b.emit(Inst{Op: OpVariant, Dst: result, Ent: some, Type: t, Args: []LocalID{val}, Node: n})
	b.jump(join)
	b.cur = no
	b.emit(Inst{Op: OpVariant, Dst: result, Ent: b.variantNamed(b.r.Types.OptionEnt(), "None"), Type: t, Node: n})
	b.jump(join)
	b.cur = join
	return result
}

func (b *builder) recordLit(e *hir.Expr) LocalID {
	info := b.r.TypeDecl(e.Ent)
	fields := make([]LocalID, len(info.Fields))
	set := make([]bool, len(info.Fields))
	for _, entry := range e.Entries {
		if !entry.Spread {
			fields[entry.Index] = b.exprMoved(entry.Value)
			set[entry.Index] = true
			continue
		}
		v := b.expr(entry.Value)
		for _, sf := range entry.Fields {
			fields[sf.Index] = b.emit(Inst{Op: OpField, Dst: b.temp(sf.Type), Args: []LocalID{v}, Index: sf.SrcIndex, Node: entry.Node})
			set[sf.Index] = true
		}
		srcType := entry.Value.Type
		if node := b.r.Types.Node(srcType); node.Kind == sem.KRef {
			srcType = node.Elem
		}
		if b.r.IsCopy(srcType) {
			// A Copy source (owned or borrowed) is unaffected: its fields
			// were read, not taken, so it stays usable.
			continue
		}
		// The spread source is consumed: its fields now belong to the new
		// record, so the source is moved and its shell released.
		moved := b.emit(Inst{Op: OpMove, Dst: b.temp(entry.Value.Type), Args: []LocalID{v}, Node: entry.Node})
		b.emit(Inst{Op: OpDrop, Args: []LocalID{moved}, Node: entry.Node})
	}
	for i := range fields {
		if !set[i] {
			fields[i] = b.unit(e.Node)
		}
	}
	return b.emit(Inst{Op: OpRecord, Dst: b.temp(e.Type), Ent: e.Ent, Type: e.Type, Args: fields, Node: e.Node})
}

// lambda lowers the closure body as its own function and captures the
// locals it uses (cells by reference, others by value).
func (b *builder) lambda(e *hir.Expr) LocalID {
	var caps []LocalID
	for _, cap := range e.Fn.Captures {
		id := b.local(cap.Ent)
		t := b.fn.Locals[id].Type
		switch {
		case b.fn.Locals[id].Cell || cap.Cell && !cap.Move:
			caps = append(caps, id)
		case cap.Cell:
			moved := b.emit(Inst{Op: OpMove, Dst: b.temp(t), Args: []LocalID{id}, Node: e.Node})
			cell := b.temp(t)
			b.fn.Locals[cell].Cell = true
			caps = append(caps, b.emit(Inst{Op: OpNewCell, Dst: cell, Args: []LocalID{moved}, Node: e.Node}))
		case cap.Move:
			caps = append(caps, b.emit(Inst{Op: OpMove, Dst: b.temp(t), Args: []LocalID{id}, Node: e.Node}))
		default:
			caps = append(caps, b.emit(Inst{Op: OpCopy, Dst: b.temp(t), Args: []LocalID{id}, Node: e.Node}))
		}
	}
	if _, built := b.p.ByEnt[e.Ent]; !built {
		b.p.build(e.Fn)
	}
	return b.emit(Inst{Op: OpClosure, Dst: b.temp(e.Type), Ent: e.Ent, Args: caps, Node: e.Node})
}
