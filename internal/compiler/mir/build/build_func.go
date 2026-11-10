package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

func (p *lowering) build(hf *hir.Func) *midir.Func {
	f := &midir.Func{Name: p.FuncName(hf.Ent), Ent: hf.Ent, File: hf.File, Ret: hf.Ret, Async: ownquery.IsAsyncBody(p.R, hf.Ent)}
	b := &builder{p: p, r: p.R, fn: f, locals: map[sem.EntityID]midir.LocalID{}, shared: map[midir.LocalID]bool{}, unowned: map[midir.LocalID]bool{}, ret: hf.Ret}
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
		// A copyable local kept in a cell for its own `&mut` but captured by copy arrives as a plain value (lambda).
		if b.r.IsMutSelf(d.Ent) || d.Cell {
			f.Locals[id].Cell = true
		} else if !d.Move && b.r.IsCopy(d.Type) {
			f.Locals[id].Cell = false
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
		var v midir.LocalID
		if hf.Body.Kind == hir.BlockExpr {
			b.body = hf.Body.Block
			v = b.detach(b.expr(hf.Body))
		} else {
			v = b.fullValue(func() midir.LocalID { return b.detach(b.expr(hf.Body)) })
		}
		if b.body == nil {
			b.closeTaskScope(hf.Body.Node)
		}
		b.exitScope()
		b.returnValue(v)
	}
	p.Funcs = append(p.Funcs, f)
	p.ByEnt[hf.Ent] = f
	p.top[f] = p.cur
	b.lowerGhosts()
	b.ghostRuns.hoistRefReleases(p.input, f)
	analyzeOwnership(p.input, f, &b.ghostRuns, p.noteMoves)
	b.ghostRuns.drop(f)
	p.checkBorrowsOf(f)
	return f
}
