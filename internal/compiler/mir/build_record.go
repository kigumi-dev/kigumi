package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (b *builder) optField(e *hir.Expr) midir.LocalID {
	n, t := e.Node, e.Type
	inner := b.r.Types.Node(t).Args[0]
	var base midir.LocalID
	if e.Ent != 0 {
		base = b.methodReceiver(e, inner)
	} else {
		base = b.expr(e.Args[0])
	}
	some := b.variantNamed(b.r.Types.OptionEnt(), "Some")
	isSome := b.emit(midir.Inst{Op: midir.OpIsVariant, Dst: b.temp(sem.TyBool), Ent: some, Args: []midir.LocalID{base}, Node: n})
	result := b.temp(t)
	yes, no, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{isSome}, Targets: []midir.BlockID{yes, no}})
	b.cur = yes
	payload := b.emit(midir.Inst{Op: midir.OpPayload, Dst: b.temp(b.r.Types.Node(e.Args[0].Type).Args[0]), Args: []midir.LocalID{base}, Node: n})
	var val midir.LocalID
	if e.Ent != 0 {
		val = b.methodValue(e, payload, inner)
	} else {
		val = b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(inner), Args: []midir.LocalID{payload}, Index: e.Index, Node: n})
	}
	b.emit(midir.Inst{Op: midir.OpVariant, Dst: result, Ent: some, Type: t, Args: []midir.LocalID{val}, Node: n})
	b.jump(join)
	b.cur = no
	b.emit(midir.Inst{Op: midir.OpVariant, Dst: result, Ent: b.variantNamed(b.r.Types.OptionEnt(), "None"), Type: t, Node: n})
	b.jump(join)
	b.cur = join
	return result
}

func (b *builder) recordLit(e *hir.Expr) midir.LocalID {
	info := b.r.TypeDecl(e.Ent)
	fields := make([]midir.LocalID, len(info.Fields))
	set := make([]bool, len(info.Fields))
	for _, entry := range e.Entries {
		if !entry.Spread {
			fields[entry.Index] = b.exprMoved(entry.Value)
			set[entry.Index] = true
			continue
		}
		v := b.expr(entry.Value)
		for _, sf := range entry.Fields {
			fields[sf.Index] = b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(sf.Type), Args: []midir.LocalID{v}, Index: sf.SrcIndex, Node: entry.Node})
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
		moved := b.emit(midir.Inst{Op: midir.OpMove, Dst: b.temp(entry.Value.Type), Args: []midir.LocalID{v}, Node: entry.Node})
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{moved}, Node: entry.Node})
	}
	for i := range fields {
		if !set[i] {
			fields[i] = b.unit(e.Node)
		}
	}
	return b.emit(midir.Inst{Op: midir.OpRecord, Dst: b.temp(e.Type), Ent: e.Ent, Type: e.Type, Args: fields, Node: e.Node})
}

// lambda lowers the closure body as its own function and captures the
// locals it uses (cells by reference, others by value).
func (b *builder) lambda(e *hir.Expr) midir.LocalID {
	var caps []midir.LocalID
	for _, cap := range e.Fn.Captures {
		id := b.local(cap.Ent)
		t := b.fn.Locals[id].Type
		switch {
		// A scalar kept in a cell for its `&mut` but captured by copy hands the closure the value it holds now.
		case b.fn.Locals[id].Cell && !cap.Cell && !cap.Move && b.r.IsCopy(t):
			caps = append(caps, b.emit(midir.Inst{Op: midir.OpCellGet, Dst: b.temp(t), Args: []midir.LocalID{id}, Node: e.Node}))
		case b.fn.Locals[id].Cell || cap.Cell && !cap.Move:
			caps = append(caps, id)
		case cap.Cell:
			moved := b.emit(midir.Inst{Op: midir.OpMove, Dst: b.temp(t), Args: []midir.LocalID{id}, Node: e.Node})
			cell := b.temp(t)
			b.fn.Locals[cell].Cell = true
			caps = append(caps, b.emit(midir.Inst{Op: midir.OpNewCell, Dst: cell, Args: []midir.LocalID{moved}, Node: e.Node}))
		case cap.Move:
			caps = append(caps, b.emit(midir.Inst{Op: midir.OpMove, Dst: b.temp(t), Args: []midir.LocalID{id}, Node: e.Node}))
		default:
			caps = append(caps, b.emit(midir.Inst{Op: midir.OpCopy, Dst: b.temp(t), Args: []midir.LocalID{id}, Node: e.Node}))
		}
	}
	if _, built := b.p.ByEnt[e.Ent]; !built {
		b.p.build(e.Fn)
	}
	return b.emit(midir.Inst{Op: midir.OpClosure, Dst: b.temp(e.Type), Ent: e.Ent, Args: caps, Node: e.Node})
}
