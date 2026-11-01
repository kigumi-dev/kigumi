package build

import (
	"kigumi/internal/compiler/hir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

// rejectRecvWrite reports a later argument a snapshot or the lent view cannot cover: any write under a `self` resource receiver, or a
// replace, move or `&mut` of a place holding a `mut self` receiver; a reborrow, or a top-level `&mut` not lent as a cell, is the borrow checker's.
func (b *builder) rejectRecvWrite(e *hir.Expr, later []*hir.Expr) {
	recv := b.r.Fn(e.Ent).Recv
	if recv != sem.RecvSelf && recv != sem.RecvMut {
		return
	}
	root := recvRoot(e.Args[0])
	if root == 0 {
		return
	}
	t := b.r.Entity(root).Type
	elem, lends := b.r.MutRefElem(t)
	if lends {
		t = elem
	}
	resource := b.r.HoldsResource(t) && !b.r.IsCopy(t)
	if !resource && recv != sem.RecvMut {
		return
	}
	w := localWrites{r: b.r, ent: root, moves: true, whole: recv == sem.RecvMut, deep: resource, lends: lends}
	w.path, w.steps = placePath(e.Args[0]), placeSteps(e.Args[0])
	for _, x := range later {
		if x == nil || lends && x.Kind == hir.Local && x.Ent == root {
			continue
		}
		hit := w.expr(x)
		if x.Kind == hir.Borrow && x.Mut && rootedAt(x.Args[0], root) {
			if !b.wantsRefCell(e.Ent, argIndex(e, x)-1) {
				continue
			}
			// The callee holds this place and the receiver at once, so a place inside the receiver conflicts too.
			hit = hit || ownquery.FieldPathsOverlap(placeSteps(x.Args[0]), w.steps)
		}
		if !hit {
			continue
		}
		code, _ := sem.CodeByName("borrow-conflict")
		msg := sem.FillTemplate(code.Template, b.r.Entity(root).Name) + "; the method's receiver borrows it while this argument writes it"
		tree := b.r.Tree(b.fn.File)
		b.fn.Diags = append(b.fn.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(tree.File, tree.Span(x.Node)), Code: ownquery.DiagnosticCode(code.Num), Msg: msg})
		return
	}
}

func argIndex(e, x *hir.Expr) int {
	for i, a := range e.Args {
		if a == x {
			return i
		}
	}
	return -1
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
