package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// returnUnwinding returns v, the value of ex, after unwinding every scope; a returned
// Result that is not statically Ok runs the defer(fail) cleanups when it holds Err.
func (b *builder) returnUnwinding(v midir.LocalID, ex *hir.Expr, n syntax.NodeID) {
	b.splitOnErr(v, n, b.dynamicErr(v, ex), func(failing bool) {
		b.closeTaskScope(n)
		b.unwindTo(0, failing)
		b.returnValue(v)
	})
}

// exitBodyScope is exitScope for a function body whose tail value is returned.
func (b *builder) exitBodyScope(v midir.LocalID, ex *hir.Expr, n syntax.NodeID) {
	if !b.dynamicErr(v, ex) {
		b.exitScope()
		return
	}
	s := b.scopes[len(b.scopes)-1]
	b.scopes = b.scopes[:len(b.scopes)-1]
	join := b.newBlockAt()
	b.splitOnErr(v, n, true, func(failing bool) {
		b.unwindScope(s, failing)
		b.jump(join)
	})
	b.cur = join
}

func (b *builder) splitOnErr(v midir.LocalID, n syntax.NodeID, dynamic bool, exit func(failing bool)) {
	if !dynamic {
		exit(false)
		return
	}
	ok := b.emit(midir.Inst{Op: midir.OpIsVariant, Dst: b.temp(sem.TyBool), Ent: b.variantNamed(b.r.Types.ResultEnt(), "Ok"), Args: []midir.LocalID{v}, Node: n})
	yes, no := b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{ok}, Targets: []midir.BlockID{yes, no}})
	b.cur = no
	exit(true)
	b.cur = yes
	exit(false)
}

func (b *builder) dynamicErr(v midir.LocalID, ex *hir.Expr) bool {
	if _, _, ok := b.r.Types.IsResult(b.fn.Locals[v].Type); !ok || !b.hasFailDefers() {
		return false
	}
	return ex != nil && !b.staticOk(ex)
}

// staticOk matches the checker's rule for a success exit that may move a defer(fail)-reserved binding.
func (b *builder) staticOk(e *hir.Expr) bool {
	if _, _, ok := b.r.Types.IsResult(e.Type); !ok {
		return true
	}
	switch e.Kind {
	case hir.Wrap:
		return b.staticOk(e.Args[0])
	case hir.BlockExpr:
		return e.Block.Tail != nil && b.staticOk(e.Block.Tail)
	case hir.If, hir.IfLet:
		return e.Else != nil && (e.Then.Tail == nil || b.staticOk(e.Then.Tail)) && b.staticOk(e.Else)
	case hir.Match:
		for _, a := range e.Arms {
			if !b.staticOk(a.Body) {
				return false
			}
		}
		return true
	case hir.Variant:
		return e.Ent == b.variantNamed(b.r.Types.ResultEnt(), "Ok")
	}
	return false
}

func (b *builder) hasFailDefers() bool {
	for _, s := range b.scopes {
		for _, d := range s.defers {
			if d.Err {
				return true
			}
		}
	}
	return false
}
