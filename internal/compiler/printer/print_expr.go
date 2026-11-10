package printer

import (
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func (p *printer) exprList(list syntax.NodeID) {
	for i, e := range p.t.List(list).Enumerate() {
		if i > 0 {
			p.write(", ")
		}
		p.withIndent(func() { p.expr(e.ID()) })
	}
}

func (p *printer) expr(id syntax.NodeID) {
	if id == 0 {
		return
	}
	p.leadingInline(id)
	p.exprNode(id)
	p.trailingInline(id)
}

func (p *printer) exprNode(id syntax.NodeID) {
	v := p.t.View(id)
	switch v.Kind() {
	case syntax.Ident, syntax.IntLit, syntax.FloatLit, syntax.CharLit, syntax.BoolLit:
		p.write(v.Tok().Text())
	case syntax.StringLit, syntax.ByteStringLit:
		p.write(v.Tok().Text())
	case syntax.ShellLit:
		p.write("$" + v.AsShellLit().Str().Text())
	case syntax.Paren:
		inner := v.AsParen().Inner()
		if inner.Kind() == syntax.Paren || inner.Kind() == syntax.Ident && inner.Tok().Text() == "_" {
			p.expr(inner.ID())
			return
		}
		p.write("(")
		p.inlineBlocks(p.firstTok(id)+1, p.firstTok(inner.ID()), false, true)
		p.expr(inner.ID())
		p.inlineBlocks(p.lastTok(inner.ID())+1, p.lastTok(id), true, false)
		p.write(")")
	case syntax.Unary:
		u := v.AsUnary()
		p.write(u.Op().Text())
		p.prefixSep(u.Operand().ID())
		p.expr(u.Operand().ID())
	case syntax.Binary:
		p.binary(id)
	case syntax.IsExpr:
		is := v.AsIs()
		p.expr(is.Left().ID())
		p.write(" is ")
		// A record or or-pattern starts with `{`; without the grouping a following block
		// (`if`, `for`) reads the brace as the pattern's body and the output stops parsing.
		pat := is.Pattern().ID()
		group := p.t.Kind(pat) == syntax.PatRecord || p.t.Kind(pat) == syntax.PatOr
		if group {
			p.write("(")
		}
		p.pattern(pat)
		if group {
			p.write(")")
		}
	case syntax.CallExpr:
		c := v.AsCall()
		p.expr(c.Callee().ID())
		p.callArgs(id, c.Callee().ID(), c.Args())
	case syntax.WsCallExpr:
		p.wsCall(id)
	case syntax.BracketExpr:
		b := v.AsBracket()
		p.expr(b.Base().ID())
		p.write("[")
		p.exprList(b.Args().ID())
		p.write("]")
	case syntax.TypeFn, syntax.TypePtr:
		p.typ(id)
	case syntax.MemberExpr:
		m := v.AsMember()
		p.expr(m.Base().ID())
		p.member(m.Member(), ".")
	case syntax.OptMemberExpr:
		m := v.AsOptMember()
		p.expr(m.Base().ID())
		p.member(m.Member(), "?.")
	case syntax.Spread:
		p.write("...")
		p.expr(v.AsSpread().Value().ID())
	case syntax.NamedArg:
		a := v.AsNamedArg()
		p.write(a.Name().Text() + ": ")
		p.expr(a.Value().ID())
	case syntax.TryExpr:
		p.expr(v.AsTry().Operand().ID())
		p.write("?")
	case syntax.RecordLit:
		p.recordLit(id)
	case syntax.TupleLit:
		p.tupleLit(id)
	case syntax.UnitLit:
		p.write("()")
	case syntax.Lambda:
		p.lambda(id)
	case syntax.Block:
		p.block(id)
	case syntax.IfExpr:
		p.ifExpr(id)
	case syntax.IfLetExpr:
		p.ifLet(id)
	case syntax.MatchExpr:
		p.matchExpr(id)
	case syntax.ForExpr:
		p.forExpr(id)
	case syntax.ReturnExpr, syntax.FailExpr, syntax.BreakExpr, syntax.ContinueExpr,
		syntax.AwaitExpr, syntax.ComptimeExpr, syntax.UnsafeExpr, syntax.MoveExpr:
		p.write(v.Tok().Text())
		if operand := p.keywordOperand(v); operand.Valid() {
			p.write(" ")
			p.expr(operand.ID())
		}
	case syntax.AsmExpr:
		p.asmBlock(id)
	case syntax.AllocatorExpr:
		a := v.AsAllocator()
		p.write("allocator ")
		p.expr(a.Scope().ID())
		p.write(" ")
		p.block(a.Body().ID())
	case syntax.ContextExpr:
		c := v.AsContext()
		p.write("context (")
		p.exprList(c.Provides().ID())
		p.write(") ")
		p.block(c.Body().ID())
	case syntax.ContractExpr:
		c := v.AsContract()
		p.write(modsText(c.Mods()) + " ")
		p.block(c.Body().ID())
	case syntax.BorrowExpr:
		b := v.AsBorrow()
		p.write("&")
		if b.Flags()&syntax.FlagMut != 0 {
			p.write("mut ")
		} else {
			p.prefixSep(b.Operand().ID())
		}
		p.expr(b.Operand().ID())
	}
}

// prefixSep writes a space when a prefix operator would otherwise fuse with the operand's first
// token (`- -n` printed as `--n` is a different token). A lifetime stays joined: `&'a T`.
func (p *printer) prefixSep(next syntax.NodeID) {
	text := p.t.View(next).Tok().Text()
	if text == "" || text[0] == '\'' {
		return
	}
	switch text[0] {
	case '!', '~', '*', '%', '+', '-', '=', '<', '>', '&', '^', '|':
		p.write(" ")
	}
}

// keywordOperand is the optional operand of a keyword expression such as
// `return x` or `await x`; `continue` has none.
func (p *printer) keywordOperand(v syntax.View) syntax.View {
	switch v.Kind() {
	case syntax.ReturnExpr:
		return v.AsReturn().Value().View
	case syntax.FailExpr:
		return v.AsFail().Value().View
	case syntax.BreakExpr:
		return v.AsBreak().Value().View
	case syntax.AwaitExpr:
		return v.AsAwait().Operand().View
	case syntax.ComptimeExpr:
		return v.AsComptime().Body().View
	case syntax.UnsafeExpr:
		return v.AsUnsafe().Body().View
	case syntax.MoveExpr:
		return v.AsMove().Operand().View
	}
	return syntax.View{}
}

// member prints a `.name` segment. A segment that started a new line in the
// source keeps its own line, indented once for the whole chain.
func (p *printer) member(name syntax.TokenRef, dot string) {
	if nameTok := name.Index(); nameTok >= 2 && p.t.Toks[nameTok-2].Kind == token.Newline {
		if j := p.pendingLineComment(nameTok - 1); j > 0 {
			p.write(" " + p.tok(j))
			p.cm.next = int(j) + 1
		}
		p.newline()
		if !p.chain {
			p.indent++
			p.chain = true
		}
		p.flushComments(uint32(nameTok - 1))
	}
	// A block comment that ends the receiver keeps a space before the dot so
	// the two tokens do not fuse.
	if strings.HasSuffix(p.sb.String(), "*/") {
		p.write(" ")
	}
	p.write(dot + name.Text())
}

// pendingLineComment is the first unprinted line comment before limit that
// ends the line it is on, so a chain segment keeps the comment beside it.
func (p *printer) pendingLineComment(limit uint32) uint32 {
	for i := p.cm.next; i < int(limit) && i < len(p.t.Toks); i++ {
		if p.t.Toks[i].Kind != token.Comment {
			continue
		}
		if i > 0 && p.t.Toks[i-1].Kind == token.Newline {
			return 0
		}
		return uint32(i)
	}
	return 0
}
