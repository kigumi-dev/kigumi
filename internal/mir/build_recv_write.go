package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

// rejectRecvWrite reports a later argument writing the local a resource-holding receiver is borrowed from, which a snapshot cannot cover;
// a plain `&mut` of it is left to the borrow checker.
func (b *builder) rejectRecvWrite(e *hir.Expr, later []*hir.Expr) {
	w := localWrites{r: b.r, moves: true}
	switch b.r.Fn(e.Ent).Recv {
	case sem.RecvSelf:
	case sem.RecvMut:
		w.whole = true
	default:
		return
	}
	root := recvRoot(e.Args[0])
	if root == 0 {
		return
	}
	t := b.r.Entity(root).Type
	if elem, _ := b.r.MutRefElem(t); elem != 0 {
		t = elem
	}
	if !b.r.HoldsResource(t) || b.r.IsCopy(t) {
		return
	}
	w.ent, w.path = root, placePath(e.Args[0])
	for _, x := range later {
		if x == nil || x.Kind == hir.Borrow && x.Mut && rootedAt(x.Args[0], root) || !w.expr(x) {
			continue
		}
		code, _ := sem.CodeByName("borrow-conflict")
		msg := sem.FillTemplate(code.Template, b.r.Entity(root).Name) + "; the method's receiver borrows it while this argument writes it"
		tree := b.r.Tree(b.fn.File)
		b.fn.Diags = append(b.fn.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(tree.File, tree.Span(x.Node)), Code: diagCode(code.Num), Msg: msg})
		return
	}
}

func recvRoot(e *hir.Expr) sem.EntityID {
	for e != nil {
		switch e.Kind {
		case hir.Local:
			return e.Ent
		case hir.Field, hir.Index, hir.Wrap, hir.DefinedWrap, hir.DefinedUnwrap:
			e = e.Args[0]
		default:
			return 0
		}
	}
	return 0
}
