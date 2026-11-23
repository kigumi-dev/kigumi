package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// protocolCursor walks a value whose type defines `iter(self) -> I` and
// `I.next(mut self) -> Item?`. iter runs once in the loop's preheader, next
// runs at the top of every iteration, and the iterator state is owned by the
// loop's bookkeeping scope so break, return and the normal exit drop it once.
type protocolCursor struct {
	b       *builder
	it      midir.LocalID
	nextv   midir.LocalID
	item    sem.TypeID
	next    sem.EntityID
	discard bool
	iterCtx []*hir.Expr
	nextCtx []*hir.Expr
	// replaces marks a `next` that may assign `self = ...`; its receiver then
	// needs an owning cell and a write-back into the loop's iterator local.
	replaces bool
}

func (b *builder) protocolCursor(e *hir.Expr) *protocolCursor {
	n := e.Node
	var it midir.LocalID
	// iter borrows the head, so a temporary head releases at the end of this
	// full expression instead of being moved into the loop.
	b.fullExpr(func() {
		head := b.expr(e.Cond)
		args := []midir.LocalID{head}
		args = append(args, b.ctxArgList(e.IterCtxArgs)...)
		it = b.emit(midir.Inst{Op: midir.OpCall, Dst: b.temp(e.IterType), Ent: e.IterEnt, Args: args, Node: n})
	})
	b.own(it)
	return &protocolCursor{
		b: b, it: it, item: e.Pat.Type, next: e.NextEnt,
		discard: e.Pat.Kind == hir.PatWild, iterCtx: e.IterCtxArgs, nextCtx: e.NextCtxArgs,
		replaces: b.r.ReplacesSelf(e.NextEnt),
	}
}

func (c *protocolCursor) needsElem() bool { return true }

func (c *protocolCursor) test(n syntax.NodeID, exit midir.BlockID) midir.LocalID {
	b := c.b
	mark := len(b.argFixups)
	itType := b.fn.Locals[c.it].Type
	rt := b.r.Types.Ref(itType, true)
	var cell midir.LocalID
	if c.replaces {
		// `next` may leave another value in `self`, so the cell owns its
		// reference and the call's write-back returns the new iterator to
		// the loop-owned local, just like an ordinary `mut self` call.
		cell = b.lendCell(c.it, rt, n, true)
		p := place{src: &hir.Place{Node: n}, id: c.it}
		b.argFixups = append(b.argFixups, argFixup{recv: &p, elemType: itType, retag: itType, cell: cell, node: n, depth: len(b.scopes)})
	} else {
		// `next` only mutates the iterator in place, so a lent frame cell is a
		// view: it owns no count and the loop's iterator local drops after it.
		ref := b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(rt), Args: []midir.LocalID{c.it}, Index: 1, Node: n})
		cell = b.emit(midir.Inst{Op: midir.OpNewCell, Dst: b.temp(rt), Args: []midir.LocalID{ref}, Node: n, Scoped: true, Lent: true})
	}
	args := []midir.LocalID{cell}
	args = append(args, b.ctxArgList(c.nextCtx)...)
	opt := b.r.Types.Option(c.item)
	c.nextv = b.emit(midir.Inst{Op: midir.OpCall, Dst: b.temp(opt), Ent: c.next, Args: args, Node: n})
	b.applyArgFixups(mark)
	isSome := b.emit(midir.Inst{Op: midir.OpIsVariant, Dst: b.temp(sem.TyBool), Ent: b.variantNamed(b.r.Types.OptionEnt(), "Some"), Args: []midir.LocalID{c.nextv}, Node: n})
	some, none, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{isSome}, Targets: []midir.BlockID{some, none}})
	// None leaves through exit directly: joining a dropped Option with the
	// Some path would make elem's payload read look like a use after drop.
	b.cur = none
	b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{c.nextv}, Node: n})
	b.jump(exit)
	b.cur = some
	b.jump(join)
	b.cur = join
	return isSome
}

func (c *protocolCursor) elem(n syntax.NodeID) midir.LocalID {
	b := c.b
	payload := b.emit(midir.Inst{Op: midir.OpPayload, Dst: b.temp(c.item), Args: []midir.LocalID{c.nextv}, Index: 0, Node: n})
	// Detach the payload before releasing the Option: the copy/move is what
	// the binding reads, so the drop cannot free it first.
	op := midir.OpMove
	if b.r.IsCopy(c.item) {
		op = midir.OpCopy
	}
	item := b.emit(midir.Inst{Op: op, Dst: b.temp(c.item), Args: []midir.LocalID{payload}, Node: n})
	b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{c.nextv}, Node: n})
	if c.discard && !b.r.IsCopy(c.item) {
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{item}, Node: n})
	}
	return item
}

// next runs at the top of every iteration, so advance has nothing left to do.
func (c *protocolCursor) advance(syntax.NodeID, midir.BlockID, midir.BlockID) {}
