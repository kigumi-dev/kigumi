package printer

import "kigumi/internal/syntax"

// separateDecls puts a blank line between declarations, except between two
// compact ones: imports, consts, statements, and bodiless fn/type declarations
// such as interface requirements and extern symbols. A doc comment makes a
// declaration a paragraph of its own, so documented ones are never glued.
func (p *printer) separateDecls(prev, next syntax.NodeID) {
	if p.compact(prev) && p.compact(next) && !p.documented(prev) && !p.documented(next) {
		if p.blankBefore(int(p.firstTok(next))) {
			p.blank()
		}
		return
	}
	p.blank()
}

func (p *printer) documented(id syntax.NodeID) bool { return p.t.View(id).AsDecl().Doc().Valid() }

func (p *printer) compact(id syntax.NodeID) bool {
	switch p.t.Kind(id) {
	case syntax.ImportDecl, syntax.ConstDecl, syntax.StaticDecl, syntax.ContextKeyDecl, syntax.LetStmt, syntax.ExprStmt,
		syntax.AssignStmt, syntax.DeferStmt, syntax.ErrdeferStmt, syntax.InstantiateDecl:
		return true
	case syntax.FnDecl:
		return !p.t.View(id).AsFnDecl().Body().Valid()
	case syntax.TypeDecl:
		return !p.t.View(id).AsTypeDecl().Body().Valid()
	}
	return false
}
