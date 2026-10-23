package hir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (l *lowerer) exprRaw(n syntax.NodeID) *Expr {
	if ch, ok := l.info.OptChains[n]; ok {
		return l.optChain(n, ch)
	}
	v := l.t.View(n)
	switch v.Kind() {
	case syntax.Ident:
		return l.ident(n)
	case syntax.IntLit, syntax.FloatLit:
		return l.literal(n)
	case syntax.CharLit:
		r, _ := syntax.DecodeChar(v.Tok().Text())
		out := l.node(n, Lit)
		out.Type, out.Lit = sem.TyChar, sem.Literal{Kind: sem.LitChar, Char: r}
		return out
	case syntax.BoolLit:
		out := l.node(n, Lit)
		out.Type, out.Lit = sem.TyBool, sem.Literal{Kind: sem.LitBool, Bool: v.Tok().Text() == "true"}
		return out
	case syntax.StringLit:
		return l.stringLit(n)
	case syntax.ByteStringLit:
		return l.byteStringLit(n)
	case syntax.ShellLit:
		return l.shellLit(n)
	case syntax.Paren:
		return l.wrap(n, v.AsParen().Inner().ID())
	case syntax.UnsafeExpr:
		return l.wrap(n, v.AsUnsafe().Body().ID())
	case syntax.Spread:
		return l.wrap(n, v.AsSpread().Value().ID())
	case syntax.Unary:
		u := v.AsUnary()
		if lit, ok := l.r.LiteralOf(l.t, n); ok && lit.Kind != sem.LitString {
			return l.literal(n)
		}
		out := l.node(n, Unary)
		out.Name, out.Args = u.Op().Text(), []*Expr{l.expr(u.Operand().ID())}
		if call, ok := l.info.Calls[n]; ok && call.Kind == sem.CallOperator {
			out.Ent = call.Callee
		}
		return out
	case syntax.Binary:
		return l.binary(n)
	case syntax.IsExpr:
		is := v.AsIs()
		out := l.node(n, Is)
		out.Args, out.Pat = []*Expr{l.expr(is.Left().ID())}, l.pattern(is.Pattern().ID())
		return out
	case syntax.CallExpr, syntax.WsCallExpr:
		return l.call(n)
	case syntax.DefaultArg:
		return l.defaultArg(n)
	case syntax.BracketExpr:
		b := v.AsBracket()
		if ent := l.info.Uses[n]; ent != 0 && l.r.Entity(ent).Kind == sem.EntFn {
			return l.fnItem(n, ent)
		}
		base, idx := b.Base().ID(), b.Args().At(0).ID()
		if call, ok := l.info.Calls[base]; ok && call.Kind == sem.CallMethodValue {
			return l.expr(base)
		}
		if call, ok := l.info.Calls[n]; ok && call.Kind == sem.CallIndexWitness {
			out := l.node(n, MethodCall)
			out.Ent, out.Args = call.Callee, []*Expr{l.expr(base), l.expr(idx)}
			out.Witness, out.Passes, out.ConstArgs = call.Witness, call.Passes, call.ConstArgs
			out.Async = l.r.Fn(call.Callee).Declared&sem.EffAsync != 0
			out.IndexWitness = true
			return out
		}
		out := l.node(n, Index)
		out.Args = []*Expr{l.expr(base), l.expr(idx)}
		return out
	case syntax.MemberExpr:
		return l.member(n)
	case syntax.OptMemberExpr:
		out := l.node(n, OptField)
		out.Args = []*Expr{l.expr(v.AsOptMember().Base().ID())}
		if call, ok := l.info.Calls[n]; ok && call.Kind == sem.CallMethodValue {
			out.Ent, out.Witness, out.Passes = call.Callee, call.Witness, call.Passes
			return out
		}
		out.Index = l.r.Field(l.info.Uses[n]).Index
		return out
	case syntax.TryExpr:
		out := l.node(n, Try)
		out.Args = []*Expr{l.expr(v.AsTry().Operand().ID())}
		if call, ok := l.info.Calls[n]; ok && len(call.Inst) == 1 {
			out.BoxType = call.Inst[0]
		}
		return out
	case syntax.RecordLit:
		return l.record(n)
	case syntax.TupleLit:
		return l.tupleLit(n)
	case syntax.UnitLit:
		return l.node(n, Unit)
	case syntax.Lambda:
		out := l.node(n, Lambda)
		out.Ent = l.info.Defs[n]
		out.Fn = l.closure(out.Ent, v.AsLambda().Body().ID())
		return out
	case syntax.Block:
		out := l.node(n, BlockExpr)
		out.Block = l.block(n)
		return out
	case syntax.IfExpr:
		s := v.AsIf()
		out := l.node(n, If)
		out.Cond, out.Then = l.expr(s.Cond().ID()), l.block(s.Then().ID())
		if els := s.Else(); els.Valid() {
			out.Else = l.expr(els.ID())
		}
		return out
	case syntax.IfLetExpr, syntax.MatchExpr, syntax.ForExpr:
		return l.control(n)
	case syntax.ReturnExpr:
		out := l.node(n, Return)
		if val := v.AsReturn().Value(); val.Valid() {
			out.Args = []*Expr{l.expr(val.ID())}
		}
		return out
	case syntax.FailExpr:
		out := l.node(n, Fail)
		out.Args = []*Expr{l.expr(v.AsFail().Value().ID())}
		return out
	case syntax.BreakExpr:
		out := l.node(n, Break)
		if val := v.AsBreak().Value(); val.Valid() {
			out.Args = []*Expr{l.expr(val.ID())}
		}
		return out
	case syntax.ContinueExpr:
		return l.node(n, Continue)
	case syntax.AsmExpr:
		out := l.node(n, Asm)
		if info := l.info.Asm[n]; info != nil {
			for _, op := range info.Operands {
				if (op.Kind == sem.AsmIn || op.Kind == sem.AsmInOut) && op.Expr != 0 {
					out.Args = append(out.Args, l.expr(op.Expr))
				}
			}
		}
		return out
	case syntax.AwaitExpr:
		out := l.node(n, Await)
		out.Args = []*Expr{l.expr(v.AsAwait().Operand().ID())}
		return out
	case syntax.ComptimeExpr:
		if lit, ok := l.r.LiteralOf(l.t, n); ok {
			out := l.node(n, Lit)
			out.Lit = lit
			return out
		}
		return l.comptime(n, v.AsComptime().Body().ID())
	case syntax.ContractExpr:
		out := l.node(n, BlockExpr)
		out.Block = l.block(v.AsContract().Body().ID())
		return out
	case syntax.AllocatorExpr:
		a := v.AsAllocator()
		out := l.node(n, Allocator)
		out.Args, out.Block = []*Expr{l.expr(a.Scope().ID())}, l.block(a.Body().ID())
		return out
	case syntax.ContextExpr:
		return l.contextExpr(n)
	case syntax.BorrowExpr:
		b := v.AsBorrow()
		out := l.node(n, Borrow)
		out.Args, out.Mut = []*Expr{l.expr(b.Operand().ID())}, b.Flags()&syntax.FlagMut != 0
		return out
	case syntax.MoveExpr:
		return l.move(n, v.AsMove().Operand().ID())
	}
	out := l.node(n, Panic)
	out.Name = "unsupported expression " + v.Kind().String()
	return out
}
