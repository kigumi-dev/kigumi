package hir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

type lowerer struct {
	r    *sem.Result
	f    sem.FileID
	t    *syntax.Tree
	info *sem.FileInfo
}

// Lower turns a checked function, implicit main or test into HIR, or nil
// when the function has no body.
func Lower(r *sem.Result, ent sem.EntityID) *Func {
	if !Lowerable(r, ent) {
		return nil
	}
	e := r.Entity(ent)
	l := &lowerer{r: r, f: e.File, t: r.Tree(e.File)}
	l.info = r.File(l.t)
	out := &Func{Ent: ent, File: e.File, Name: r.Packages[e.Pkg].Path + "." + e.Name}
	failing := r.Types.Result(sem.TyUnit, r.Types.Iface(r.Types.ErrorEnt(), nil))
	switch e.Kind {
	case sem.EntFn:
		info := r.Fn(ent)
		out.Kind = FuncFn
		if info.SelfParam != 0 {
			out.Self = l.decl(info.SelfParam)
			out.SelfMove = info.Recv == sem.RecvMove
			owner := info.Owner
			out.Destructor = owner != 0 && r.Entity(owner).Kind == sem.EntType && r.TypeDecl(owner).Drop == ent
		}
		for _, p := range info.Params {
			out.Params = append(out.Params, l.decl(p))
		}
		for _, w := range info.Witnesses {
			out.Witness = append(out.Witness, l.decl(w.Local))
		}
		for _, local := range info.ConstParams {
			out.Const = append(out.Const, l.decl(local))
		}
		for _, key := range info.CtxRequires {
			if info.CtxReal[key] {
				out.CtxParams = append(out.CtxParams, l.decl(info.CtxParam[key]))
			}
		}
		out.Ret = r.Types.Node(info.Sig).Elem
		out.Body = l.expr(info.Body)
	case sem.EntImplicitMain:
		out.Kind, out.Ret, out.Name = FuncMain, failing, "main"
		out.Main = &Block{Type: sem.TyUnit, Node: l.t.Root}
		for d := range l.t.List(l.t.Root).IDs() {
			if isStatement(l.t.Kind(d)) {
				out.Main.Stmts = append(out.Main.Stmts, l.stmt(d))
			}
		}
	case sem.EntTest:
		out.Kind, out.Ret, out.Name = FuncTest, failing, "test."+e.Name
		out.Body = l.expr(l.t.View(e.Node).AsTestDecl().Body().ID())
	default:
		return nil
	}
	return out
}

// Lowerable reports whether Lower returns a body for ent.
func Lowerable(r *sem.Result, ent sem.EntityID) bool {
	e := r.Entity(ent)
	if e.File == 0 {
		return false
	}
	switch e.Kind {
	case sem.EntFn:
		return r.Fn(ent).Body != 0 && r.Entity(e.Parent).Kind != sem.EntInterface
	case sem.EntImplicitMain, sem.EntTest:
		return true
	}
	return false
}

func isStatement(k syntax.NodeKind) bool {
	switch k {
	case syntax.LetStmt, syntax.AssignStmt, syntax.ExprStmt, syntax.DeferStmt, syntax.DeferFailStmt:
		return true
	}
	return false
}

func (l *lowerer) closure(ent sem.EntityID, body syntax.NodeID) *Func {
	info := l.r.Closure(ent)
	out := &Func{Kind: FuncClosure, Ent: ent, File: l.f, Ret: info.Ret}
	for _, cap := range info.Captures {
		d := l.decl(cap.Local)
		d.Cell, d.Move = cap.Mode == sem.CapCell || cap.OwnCell, cap.Mode == sem.CapMove
		out.Captures = append(out.Captures, d)
	}
	for _, p := range info.Params {
		out.Params = append(out.Params, l.decl(p))
	}
	out.Body = l.expr(body)
	return out
}

func (l *lowerer) decl(ent sem.EntityID) *LocalDecl {
	e := l.r.Entity(ent)
	return &LocalDecl{Ent: ent, Name: e.Name, Type: e.Type, Mut: e.Flags&sem.EfMut != 0}
}

func (l *lowerer) loc(n syntax.NodeID) diag.Location { return diag.At(l.t.File, l.t.Span(n)) }

func (l *lowerer) typeOf(n syntax.NodeID) sem.TypeID { return l.info.Types[n] }

func (l *lowerer) coercion(n syntax.NodeID) Coercion {
	co, ok := l.info.Coerce[n]
	if !ok {
		return Coercion{}
	}
	out := Coercion{Coerce: co.Steps, CoFrom: co.From}
	for _, s := range co.Steps {
		if s.Kind == sem.CoCFnPtr {
			out.CoEnt = l.info.Uses[n]
		}
	}
	return out
}

// block lowers a `{ ... }`: statements, then the last expression
// statement as the value.
func (l *lowerer) block(n syntax.NodeID) *Block {
	b := &Block{Type: l.typeOf(n), Node: n, Coercion: l.coercion(n)}
	stmts := l.t.List(n)
	for i, sv := range stmts.Enumerate() {
		if i == stmts.Len()-1 && sv.Kind() == syntax.ExprStmt && !l.discardedTail(b.Type, sv.ID()) {
			b.Tail = l.expr(sv.AsExprStmt().Expr().ID())
			break
		}
		b.Stmts = append(b.Stmts, l.stmt(sv.ID()))
	}
	return b
}

func (l *lowerer) stmt(n syntax.NodeID) *Stmt {
	v := l.t.View(n)
	out := &Stmt{Loc: l.loc(n), Node: n}
	switch v.Kind() {
	case syntax.LetStmt:
		let := v.AsLet()
		out.Kind = LetStmt
		out.Expr = l.expr(let.Init().ID())
		out.Pat = l.pattern(let.Pattern().ID())
		if els := let.Else(); els.Valid() {
			out.Else = l.block(els.ID())
		}
	case syntax.AssignStmt:
		return l.assign(n, out)
	case syntax.ExprStmt:
		out.Kind, out.Expr = ExprStmt, l.expr(v.AsExprStmt().Expr().ID())
	case syntax.DeferStmt, syntax.DeferFailStmt:
		out.Kind, out.Err = DeferStmt, v.Kind() == syntax.DeferFailStmt
		body := l.deferBody(v)
		if l.t.Kind(body) == syntax.Block {
			out.Block = l.block(body)
		} else {
			out.Body = l.stmt(body)
		}
	default:
		out.Kind, out.Expr = EvalStmt, l.expr(n)
	}
	return out
}

func (l *lowerer) deferBody(v syntax.View) syntax.NodeID {
	if v.Kind() == syntax.DeferFailStmt {
		return v.AsDeferFail().Body().ID()
	}
	return v.AsDefer().Body().ID()
}

func (l *lowerer) assign(n syntax.NodeID, out *Stmt) *Stmt {
	a := l.t.View(n).AsAssign()
	lhs, rhs := a.Target().ID(), a.Value().ID()
	if l.t.Kind(lhs) == syntax.Ident && l.t.View(lhs).Tok().Text() == "_" {
		out.Kind, out.Expr = DiscardStmt, l.expr(rhs)
		return out
	}
	out.Kind = AssignStmt
	out.Place = l.place(lhs)
	if call, ok := l.info.Calls[n]; ok && (call.Kind == sem.CallBuiltinOp || call.Kind == sem.CallOperator) {
		out.Compound, out.OpText = true, call.OpText
		if call.Kind == sem.CallOperator {
			out.OpEnt = call.Callee
		}
	}
	out.Expr = l.expr(rhs)
	return out
}
