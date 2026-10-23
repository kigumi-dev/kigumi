package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/hir"
	"kigumi/internal/sem"
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
				b.own(in.Args[0])
			}
			return
		}
	}
}

// borrowedIsTemp keeps a condition's `&mk() is P(x)` temporary for its bindings; behind `&&`
// the temporary may never be created, so a binding into it there is rejected instead.
func (b *builder) borrowedIsTemp(e *hir.Expr, v LocalID) {
	if !b.condIs[e.Pat] || !borrowsTemporary(e.Args[0]) {
		return
	}
	if b.shortRHS == 0 {
		b.ownBorrowedTemp(e.Args[0], v)
		return
	}
	if t := b.r.Types.Node(e.Args[0].Type).Elem; b.r.Types.Kind(t) == sem.KPrim && t != sem.TyString && t != sem.TyBytes {
		return
	}
	if bind := innerBinding(e.Pat); bind != nil {
		code, _ := sem.CodeByName("temporary-borrow-kept")
		msg := sem.FillTemplate(code.Template, b.r.Entity(bind.Ent).Name, "that the right side of `&&` creates only when the left side holds") + "; " + code.Help
		t := b.r.Tree(b.fn.File)
		b.fn.Diags = append(b.fn.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(bind.Node)), Code: diagCode(code.Num), Msg: msg})
	}
}

// innerBinding finds a binding below p's top level; binding the whole borrow counts the temporary.
func innerBinding(p *hir.Pat) *hir.Pat {
	var walk func(q *hir.Pat, top bool) *hir.Pat
	walk = func(q *hir.Pat, top bool) *hir.Pat {
		if q == nil {
			return nil
		}
		if q.Kind == hir.PatBind && !top {
			return q
		}
		for _, s := range q.Subs {
			if r := walk(s, q.Kind == hir.PatOr && top); r != nil {
				return r
			}
		}
		for _, f := range q.Fields {
			if r := walk(f.Sub, false); r != nil {
				return r
			}
		}
		return nil
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

// handOverBindings moves an `if` condition's bindings to the enclosing scope on the else path;
// a scrutinee temporary has no ownership tracking to skip a second drop there, so it goes now.
func (b *builder) handOverBindings(outer, bindings *scope) {
	for _, l := range bindings.owned {
		if b.fn.Locals[l].Ent == 0 {
			b.emit(Inst{Op: OpDrop, Args: []LocalID{l}, Node: b.dropNode(l), EndScope: true})
			continue
		}
		outer.owned = append(outer.owned, l)
	}
}
