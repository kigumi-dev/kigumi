package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

func (p *Program) build(hf *hir.Func) *Func {
	f := &Func{Name: p.funcName(hf.Ent), Ent: hf.Ent, File: hf.File, Ret: hf.Ret, Async: p.isAsyncBody(hf.Ent)}
	b := &builder{p: p, r: p.R, fn: f, locals: map[sem.EntityID]LocalID{}, shared: map[LocalID]bool{}, unowned: map[LocalID]bool{}, ret: hf.Ret}
	b.newBlock()
	b.pushScope()
	if hf.Self != nil {
		// A `mut self` receiver arrives as the caller's write-back cell; every
		// other self goes through param(), like an ordinary parameter.
		if b.r.IsMutSelf(hf.Self.Ent) {
			id := b.declare(hf.Self.Ent)
			f.Locals[id].Cell = true
			f.Params = append(f.Params, id)
		} else {
			f.Params = append(f.Params, b.param(hf.Self))
		}
		self := b.locals[hf.Self.Ent]
		f.Locals[self].Borrowed = !hf.SelfMove
		// A self/mut self receiver is borrowed from the caller, and a
		// destructor's move self is already being destroyed: neither is dropped here.
		if !hf.SelfMove || hf.Destructor {
			b.scopes[0].owned = nil
		}
	}
	for _, d := range hf.Captures {
		id := b.declare(d.Ent)
		f.Params = append(f.Params, id)
		// A captured mut self inherits the same borrowed-place status: EfSelf
		// without EfMove marks it, since a closure's Func has no hf.Self to read.
		e := b.r.Entity(d.Ent).Flags
		f.Locals[id].Borrowed = e&sem.EfSelf != 0 && e&sem.EfMove == 0
		if b.r.IsMutSelf(d.Ent) || d.Cell {
			f.Locals[id].Cell = true
		}
	}
	for _, decls := range [][]*hir.LocalDecl{hf.Params, hf.Witness, hf.Const} {
		for _, d := range decls {
			f.Params = append(f.Params, b.param(d))
		}
	}
	// A context key's hidden parameter is a lease, not an owned value: like
	// a borrowed `self` above, it is dropped out of the owning scope.
	ownedBefore := len(b.scopes[0].owned)
	for _, d := range hf.CtxParams {
		id := b.param(d)
		f.Params = append(f.Params, id)
		f.Locals[id].Borrowed = true
	}
	b.scopes[0].owned = b.scopes[0].owned[:ownedBefore]
	if hf.Kind == hir.FuncMain {
		for _, s := range hf.Main.Stmts {
			b.stmt(s)
		}
		b.exitScope()
		f.ImplicitOk = true
		b.returnValue(b.unit(hf.Main.Node))
	} else {
		var v LocalID
		if hf.Body.Kind == hir.BlockExpr {
			b.body = hf.Body.Block
			v = b.detach(b.expr(hf.Body))
		} else {
			v = b.fullValue(func() LocalID { return b.detach(b.expr(hf.Body)) })
		}
		if b.body == nil {
			b.closeTaskScope(hf.Body.Node)
		}
		b.exitScope()
		b.returnValue(v)
	}
	p.Funcs = append(p.Funcs, f)
	p.ByEnt[hf.Ent] = f
	if p.lowering != nil {
		p.lowering.top[f] = p.lowering.cur
	}
	b.lowerGhosts()
	hoistRefReleases(p, f)
	analyzeOwnership(p, f)
	f.dropGhosts()
	analyzeBorrows(p, f)
	checkBorrowedMoves(p, f)
	checkImmediateCaptures(p, f)
	return f
}
