package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

// ownBorrowedTemp hands the temporary a `&mk()` scrutinee points into to the current scope,
// so the pattern's bindings into it stay valid until the scope ends.
func (b *builder) ownBorrowedTemp(e *hir.Expr, ref LocalID) {
	if !borrowsTemporary(e) {
		return
	}
	insts := b.block().Insts
	for i := len(insts) - 1; i >= 0; i-- {
		if in := insts[i]; shapes[in.Op].result && in.Dst == ref {
			if in.Op == OpBorrow && b.fn.Locals[in.Args[0]].Ent == 0 {
				parents := b.projectedFrom(in.Args[0], map[LocalID]bool{}, nil)
				for k := len(parents) - 1; k >= 0; k-- {
					b.own(parents[k])
				}
				b.own(in.Args[0])
			}
			return
		}
	}
}

// borrowedIsTemp keeps a temporary an `is` binding borrows through; behind `&&` it may never be created, so such a binding is rejected.
func (b *builder) borrowedIsTemp(e *hir.Expr, v LocalID) bool {
	if !b.condIs[e.Pat] && !b.valueIs[e.Pat] {
		return false
	}
	if b.countedRefTemp(v) {
		bind, at := innerBinding(e.Pat, func(ent sem.EntityID) bool { return b.r.Types.Kind(b.r.Entity(ent).Type) == sem.KRef })
		if bind == 0 {
			return false
		}
		b.own(v)
		if b.shortRHS > 0 {
			b.tempBorrowKept(bind, at)
		}
		return true
	}
	if !borrowsTemporary(e.Args[0]) {
		return false
	}
	if b.shortRHS == 0 {
		b.ownBorrowedTemp(e.Args[0], v)
		return false
	}
	if t := b.r.Types.Node(e.Args[0].Type).Elem; b.r.Types.Kind(t) == sem.KPrim && t != sem.TyString && t != sem.TyBytes {
		return false
	}
	if bind, at := innerBinding(e.Pat, func(sem.EntityID) bool { return true }); bind != 0 {
		b.tempBorrowKept(bind, at)
	}
	return false
}

func (b *builder) tempBorrowKept(bind sem.EntityID, at syntax.NodeID) {
	code, _ := sem.CodeByName("temporary-borrow-kept")
	msg := sem.FillTemplate(code.Template, b.r.Entity(bind).Name, "that the right side of `&&` creates only when the left side holds") + "; " + code.Help
	t := b.r.Tree(b.fn.File)
	b.fn.Diags = append(b.fn.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(at)), Code: diagCode(code.Num), Msg: msg})
}

func (b *builder) countedRefTemp(v LocalID) bool {
	l := b.fn.Locals[v]
	return l.Ent == 0 && !l.Borrowed && !b.unowned[v] && b.r.Types.Kind(l.Type) == sem.KRef
}

// innerBinding finds a binding below p's top level that want accepts; binding the whole borrow counts the temporary.
func innerBinding(p *hir.Pat, want func(sem.EntityID) bool) (sem.EntityID, syntax.NodeID) {
	var walk func(q *hir.Pat, top bool) (sem.EntityID, syntax.NodeID)
	walk = func(q *hir.Pat, top bool) (sem.EntityID, syntax.NodeID) {
		if q == nil {
			return 0, 0
		}
		if q.Kind == hir.PatBind && !top && want(q.Ent) {
			return q.Ent, q.Node
		}
		for _, s := range q.Subs {
			if r, n := walk(s, q.Kind == hir.PatOr && top); r != 0 {
				return r, n
			}
		}
		for _, f := range q.Fields {
			if f.Sub == nil && f.Bind != 0 && want(f.Bind) {
				return f.Bind, f.Node
			}
			if r, n := walk(f.Sub, false); r != 0 {
				return r, n
			}
		}
		return 0, 0
	}
	return walk(p, true)
}

func borrowsTemporary(e *hir.Expr) bool {
	for e.Kind == hir.Wrap {
		e = e.Args[0]
	}
	if e.Kind != hir.Borrow {
		return false
	}
	root := e.Args[0]
	for root.Kind == hir.Wrap || root.Kind == hir.Field || root.Kind == hir.Index || root.Kind == hir.DefinedUnwrap {
		root = root.Args[0]
	}
	switch root.Kind {
	case hir.Local, hir.Global, hir.ArgRef:
		return false
	}
	return true
}

// handOverBindings gives outer a closed scope's named bindings and drops its temporaries now, since nothing skips a temporary's second drop.
func (b *builder) handOverBindings(outer, bindings *scope) {
	for _, l := range bindings.owned {
		if b.fn.Locals[l].Ent == 0 {
			b.emit(Inst{Op: OpDrop, Args: []LocalID{l}, Node: b.dropNode(l), EndScope: true})
			continue
		}
		outer.owned = append(outer.owned, l)
	}
}
