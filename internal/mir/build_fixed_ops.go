package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

type inBounds struct {
	no, join BlockID
	idx      LocalID
}

func (b *builder) branchInBounds(e *hir.Expr, idx LocalID) inBounds {
	_, n, _ := b.r.FixedElem(e.Args[0].Type)
	ok := b.emit(Inst{Op: OpBinary, Dst: b.temp(sem.TyBool), Str: "<", Args: []LocalID{idx, b.usize(n, e)}, Node: e.Node})
	yes, no, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(Term{Op: TermBranch, Args: []LocalID{ok}, Targets: []BlockID{yes, no}})
	b.cur = yes
	return inBounds{no: no, join: join, idx: idx}
}

func (b *builder) otherwise(br inBounds, result LocalID, e *hir.Expr, fill func()) LocalID {
	b.jump(br.join)
	b.cur = br.no
	fill()
	b.jump(br.join)
	b.cur = br.join
	b.emit(Inst{Op: OpDrop, Args: []LocalID{br.idx}, Node: e.Node})
	return result
}

func (b *builder) option(dst LocalID, e *hir.Expr, variant string, args ...LocalID) {
	opt := b.r.Types.OptionEnt()
	b.emit(Inst{Op: OpVariant, Dst: dst, Ent: b.variantNamed(opt, variant), Type: e.Type, Args: args, Node: e.Node})
}

// elemCopy is a value apart from the slot, so a borrow or cell pins it without pinning the whole array.
func (b *builder) elemCopy(e *hir.Expr, fa, idx LocalID) LocalID {
	elem, _, _ := b.r.FixedElem(e.Args[0].Type)
	el := b.emit(Inst{Op: OpIndex, Dst: b.temp(elem), Args: []LocalID{fa, idx}, Node: e.Node})
	return b.emit(Inst{Op: OpShare, Dst: b.temp(elem), Args: []LocalID{el}, Copy: true, Node: e.Node})
}

func (b *builder) fixedGet(e *hir.Expr) (LocalID, bool) {
	if !b.r.FixedInline(e.Ent, e.Args[0].Type) {
		return 0, false
	}
	args, ok := b.fixedArgs(e, false)
	if !ok {
		return 0, false
	}
	elem, _, _ := b.r.FixedElem(e.Args[0].Type)
	result := b.temp(e.Type)
	br := b.branchInBounds(e, args[1])
	b.option(result, e, "Some", b.emit(Inst{Op: OpIndex, Dst: b.temp(elem), Args: []LocalID{args[0], args[1]}, Node: e.Node}))
	return b.otherwise(br, result, e, func() { b.option(result, e, "None") }), true
}

func (b *builder) fixedAt(e *hir.Expr) (LocalID, bool) {
	args, ok := b.fixedArgs(e, false)
	if !ok {
		return 0, false
	}
	b.ownLent(args[2])
	elem, _, _ := b.r.FixedElem(e.Args[0].Type)
	result := b.temp(e.Type)
	br := b.branchInBounds(e, args[1])
	ref := b.emit(Inst{Op: OpBorrow, Dst: b.temp(b.r.Types.Ref(elem, false)), Args: []LocalID{b.elemCopy(e, args[0], args[1])}, Node: e.Node})
	r := b.emit(Inst{Op: OpCallValue, Dst: b.temp(b.r.Types.Node(e.Type).Args[0]), Args: []LocalID{args[2], ref}, Node: e.Node})
	b.option(result, e, "Some", r)
	return b.otherwise(br, result, e, func() { b.option(result, e, "None") }), true
}

func (b *builder) fixedSet(e *hir.Expr) (LocalID, bool) {
	args, ok := b.fixedArgs(e, true)
	if !ok {
		return 0, false
	}
	result := b.temp(sem.TyBool)
	br := b.branchInBounds(e, args[1])
	b.emit(Inst{Op: OpSetIndex, Args: []LocalID{args[0], args[1], args[2]}, Node: e.Node})
	b.emit(Inst{Op: OpConst, Dst: result, Lit: sem.Literal{Kind: sem.LitBool, Bool: true}, Type: sem.TyBool, Node: e.Node})
	return b.otherwise(br, result, e, func() {
		b.emit(Inst{Op: OpDrop, Args: []LocalID{args[2]}, Node: e.Node})
		b.emit(Inst{Op: OpConst, Dst: result, Lit: sem.Literal{Kind: sem.LitBool}, Type: sem.TyBool, Node: e.Node})
	}), true
}

func (b *builder) fixedWithMut(e *hir.Expr) (LocalID, bool) {
	args, ok := b.fixedArgs(e, true)
	if !ok {
		return 0, false
	}
	b.ownLent(args[2])
	elem, _, _ := b.r.FixedElem(e.Args[0].Type)
	result := b.temp(e.Type)
	br := b.branchInBounds(e, args[1])
	cell := b.lendCell(b.elemCopy(e, args[0], args[1]), b.r.Types.Ref(elem, true), e.Node, true)
	r := b.emit(Inst{Op: OpCallValue, Dst: b.temp(b.r.Types.Node(e.Type).Args[0]), Args: []LocalID{args[2], cell}, Node: e.Node})
	v := b.emit(Inst{Op: OpCellTake, Dst: b.temp(elem), Args: []LocalID{cell}, Node: e.Node})
	b.emit(Inst{Op: OpSetIndex, Args: []LocalID{args[0], args[1], v}, Node: e.Node})
	b.option(result, e, "Some", r)
	return b.otherwise(br, result, e, func() { b.option(result, e, "None") }), true
}

// The callback is only lent to the call, so a temporary one is dropped with the scope.
func (b *builder) ownLent(l LocalID) {
	if b.fn.Locals[l].Ent == 0 {
		b.own(l)
	}
}
