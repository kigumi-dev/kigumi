package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type inBounds struct {
	no, join midir.BlockID
	idx      midir.LocalID
}

func (b *builder) branchInBounds(e *hir.Expr, idx midir.LocalID) inBounds {
	_, n, _ := b.r.FixedElem(e.Args[0].Type)
	ok := b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(sem.TyBool), Str: "<", Args: []midir.LocalID{idx, b.usize(n, e)}, Node: e.Node})
	yes, no, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{ok}, Targets: []midir.BlockID{yes, no}})
	b.cur = yes
	return inBounds{no: no, join: join, idx: idx}
}

func (b *builder) otherwise(br inBounds, result midir.LocalID, e *hir.Expr, fill func()) midir.LocalID {
	b.jump(br.join)
	b.cur = br.no
	fill()
	b.jump(br.join)
	b.cur = br.join
	b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{br.idx}, Node: e.Node})
	return result
}

func (b *builder) option(dst midir.LocalID, e *hir.Expr, variant string, args ...midir.LocalID) {
	opt := b.r.Types.OptionEnt()
	b.emit(midir.Inst{Op: midir.OpVariant, Dst: dst, Ent: b.variantNamed(opt, variant), Type: e.Type, Args: args, Node: e.Node})
}

// elemCopy is a value apart from the slot, so a borrow or cell pins it without pinning the whole array.
func (b *builder) elemCopy(e *hir.Expr, fa, idx midir.LocalID) midir.LocalID {
	elem, _, _ := b.r.FixedElem(e.Args[0].Type)
	el := b.emit(midir.Inst{Op: midir.OpIndex, Dst: b.temp(elem), Args: []midir.LocalID{fa, idx}, Node: e.Node, NoTrap: true})
	return b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(elem), Args: []midir.LocalID{el}, Copy: true, Node: e.Node})
}

func (b *builder) fixedGet(e *hir.Expr) (midir.LocalID, bool) {
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
	b.option(result, e, "Some", b.emit(midir.Inst{Op: midir.OpIndex, Dst: b.temp(elem), Args: []midir.LocalID{args[0], args[1]}, Node: e.Node, NoTrap: true}))
	return b.otherwise(br, result, e, func() { b.option(result, e, "None") }), true
}

func (b *builder) fixedAt(e *hir.Expr) (midir.LocalID, bool) {
	args, ok := b.fixedArgs(e, false)
	if !ok {
		return 0, false
	}
	b.ownLent(args[2])
	elem, _, _ := b.r.FixedElem(e.Args[0].Type)
	result := b.temp(e.Type)
	br := b.branchInBounds(e, args[1])
	ref := b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(b.r.Types.Ref(elem, false)), Args: []midir.LocalID{b.elemCopy(e, args[0], args[1])}, Node: e.Node})
	r := b.emit(midir.Inst{Op: midir.OpCallValue, Dst: b.temp(b.r.Types.Node(e.Type).Args[0]), Args: []midir.LocalID{args[2], ref}, Node: e.Node})
	b.option(result, e, "Some", r)
	return b.otherwise(br, result, e, func() { b.option(result, e, "None") }), true
}

func (b *builder) fixedSet(e *hir.Expr) (midir.LocalID, bool) {
	args, ok := b.fixedArgs(e, true)
	if !ok {
		return 0, false
	}
	result := b.temp(sem.TyBool)
	br := b.branchInBounds(e, args[1])
	b.emit(midir.Inst{Op: midir.OpSetIndex, Args: []midir.LocalID{args[0], args[1], args[2]}, Node: e.Node, NoTrap: true})
	b.emit(midir.Inst{Op: midir.OpConst, Dst: result, Lit: sem.Literal{Kind: sem.LitBool, Bool: true}, Type: sem.TyBool, Node: e.Node})
	return b.otherwise(br, result, e, func() {
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{args[2]}, Node: e.Node})
		b.emit(midir.Inst{Op: midir.OpConst, Dst: result, Lit: sem.Literal{Kind: sem.LitBool}, Type: sem.TyBool, Node: e.Node})
	}), true
}

func (b *builder) fixedWithMut(e *hir.Expr) (midir.LocalID, bool) {
	args, ok := b.fixedArgs(e, true)
	if !ok {
		return 0, false
	}
	b.ownLent(args[2])
	elem, _, _ := b.r.FixedElem(e.Args[0].Type)
	result := b.temp(e.Type)
	br := b.branchInBounds(e, args[1])
	cell := b.lendCell(b.elemCopy(e, args[0], args[1]), b.r.Types.Ref(elem, true), e.Node, true)
	r := b.emit(midir.Inst{Op: midir.OpCallValue, Dst: b.temp(b.r.Types.Node(e.Type).Args[0]), Args: []midir.LocalID{args[2], cell}, Node: e.Node})
	v := b.emit(midir.Inst{Op: midir.OpCellTake, Dst: b.temp(elem), Args: []midir.LocalID{cell}, Node: e.Node})
	b.emit(midir.Inst{Op: midir.OpSetIndex, Args: []midir.LocalID{args[0], args[1], v}, Node: e.Node, NoTrap: true})
	b.option(result, e, "Some", r)
	return b.otherwise(br, result, e, func() { b.option(result, e, "None") }), true
}

// The callback is only lent to the call, so a temporary one is dropped with the scope.
func (b *builder) ownLent(l midir.LocalID) {
	if b.fn.Locals[l].Ent == 0 {
		b.own(l)
	}
}
