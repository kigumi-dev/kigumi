package hir

import "kigumi/internal/syntax"

// contextExpr lowers `context (providers...) { body }` to a block that binds
// each provider expression to a hidden local once, ahead of the body.
func (l *lowerer) contextExpr(n syntax.NodeID) *Expr {
	ctx := l.t.View(n).AsContext()
	providers := ctx.Provides()
	locals := l.info.CtxProviders[n]
	body := l.block(ctx.Body().ID())
	prelude := make([]*Stmt, 0, providers.Len())
	for i, pv := range providers.Enumerate() {
		p := pv.ID()
		if i >= len(locals) || locals[i] == 0 {
			continue
		}
		local := locals[i]
		prelude = append(prelude, &Stmt{
			Kind: LetStmt,
			Loc:  l.loc(p),
			Node: p,
			Pat:  &Pat{Kind: PatBind, Node: p, Type: l.r.Entity(local).Type, Ent: local},
			Expr: l.expr(p),
		})
	}
	body.Stmts = append(prelude, body.Stmts...)
	out := l.node(n, BlockExpr)
	out.Block = body
	return out
}
