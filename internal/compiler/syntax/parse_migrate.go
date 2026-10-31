package syntax

import (
	"strings"

	"kigumi/internal/lang/token"
)

func (p *parser) tokSpan(i uint32) token.Span { return p.toks[i].Span }

func (p *parser) lineHasBrace() bool {
	for i := p.pos; int(i) < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case token.LBrace:
			return true
		case token.Newline, token.EOF:
			return false
		}
	}
	return false
}

func (p *parser) whileAhead() bool {
	switch p.peek(1) {
	case token.Assign, token.Dot, token.Colon, token.Comma, token.Newline, token.EOF, token.RBrace, token.RParen:
		return false
	}
	return p.lineHasBrace()
}

func (p *parser) whileStmt() NodeID {
	kw := p.pos
	p.errorHelp(p.tokSpan(kw), "write `for <condition> { ... }`, or `for { ... }` for a loop that never ends", "`while` does not exist")
	p.advance()
	cond := p.condition()
	return p.node1(ExprStmt, kw, p.tree.addRec(ForExpr, kw, []uint32{0, uint32(cond), uint32(p.block())}))
}

func (p *parser) varStmt() NodeID {
	kw := p.pos
	p.errorHelp(p.tokSpan(kw), "write `let mut name = value`; bindings are immutable unless marked `mut`", "`var` does not exist")
	p.advance()
	return p.letBinding(kw, FlagMut)
}

func (p *parser) nestedFn() NodeID {
	kw := p.pos
	p.errorHelp(p.tokSpan(kw), "declare the function at the top level, or bind a closure: `let name = (x) => x + 1`", "a `fn` cannot be declared inside a function body")
	p.fnDecl(prefix{})
	return 0
}

func (p *parser) elifStmt() NodeID {
	kw := p.pos
	p.elifError()
	return p.node1(ExprStmt, kw, p.ifRest(kw))
}

func (p *parser) elifError() {
	p.errorHelp(p.tokSpan(p.pos), "write `else if`", "`elif` does not exist")
	p.advance()
}

func (p *parser) foreignTypeDecl(word, form string) NodeID {
	p.errorHelp(p.tokSpan(p.pos), form, "`%s` does not exist", word)
	p.recover()
	return 0
}

func (p *parser) cStyleForAhead() bool {
	depth, braces := 0, 0
	for i := p.pos; int(i) < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case token.LParen, token.LBracket:
			depth++
		case token.RParen, token.RBracket:
			depth--
		case token.LBrace:
			if depth <= 0 {
				return false
			}
			braces++
		case token.RBrace:
			braces--
		case token.Semicolon:
			if braces <= 0 {
				return true
			}
		case token.Newline, token.EOF:
			return false
		}
	}
	return false
}

func (p *parser) cStyleFor(kw uint32) NodeID {
	p.errorHelp(p.tokSpan(kw), "iterate a range with `for i in 0..n { ... }`, or loop on a condition with `for <condition> { ... }`", "a C-style `for` does not exist")
	depth := 0
	for !p.at(token.EOF) && !p.at(token.Newline) {
		k := p.tok().Kind
		if k == token.LBrace && depth <= 0 {
			break
		}
		if k == token.LParen || k == token.LBracket {
			depth++
		} else if k == token.RParen || k == token.RBracket {
			depth--
		}
		p.pos++
	}
	if !p.at(token.LBrace) {
		return p.tree.addRec(ForExpr, kw, []uint32{0, 0, 0})
	}
	return p.tree.addRec(ForExpr, kw, []uint32{0, 0, uint32(p.block())})
}

func (p *parser) quotedImport() bool {
	if !p.at(token.String) {
		return false
	}
	lit := strings.Trim(p.text(), `"`)
	name := lit[strings.LastIndexByte(lit, '/')+1:]
	help := "write `import name from path`, naming the binding and giving the path without quotes"
	if lit != "" && validPathSegment(name) && !strings.ContainsAny(name, ".-") {
		help = "write `import " + name + " from " + lit + "`"
	}
	p.errorHelp(p.tok().Span, help, "an import path is not a string")
	p.advance()
	return true
}

// printOperand rejects `print n + 1`, which reads as `(print n) + 1`.
func (p *parser) printOperand(callee NodeID) {
	n := p.tree.nodes[callee]
	if n.Kind != Ident {
		return
	}
	name := p.toks[n.Tok].Text(p.tree.File.Src)
	if name != "print" && name != "eprint" {
		return
	}
	if prec, _ := p.binaryPrec(); prec == 0 || !p.tok().HasSpaceBefore() {
		return
	}
	p.errorHelp(p.tok().Span, "wrap the whole argument in parentheses, as in `"+name+" (n + 1)`", "`%s` takes only the one argument written right after it, so this operator would apply to the result of `%s`", name, name)
}

// incrementSuffix rejects `a++` and `a--` at the end of a line, which would otherwise take the next line as the right operand.
func (p *parser) incrementSuffix(op uint32) bool {
	t := p.toks[op]
	if t.Kind != token.Operator || t.HasSpaceBefore() {
		return false
	}
	text := t.Text(p.tree.File.Src)
	if text != "++" && text != "--" {
		return false
	}
	switch p.tok().Kind {
	case token.Newline, token.RBrace, token.RParen, token.Semicolon, token.EOF:
	default:
		return false
	}
	step := map[string]string{"++": "+=", "--": "-="}[text]
	p.errorHelp(t.Span, "write `a "+step+" 1`", "`%s` is not an operator in Kigumi", text)
	return true
}

func (p *parser) armAssignment() {
	if !p.at(token.Assign) && !p.tok().Kind.IsCompoundAssign() {
		return
	}
	p.errorHelp(p.tok().Span, "put the assignment in a block: `pattern -> { a = 1 }`", "an assignment is a statement, so it cannot be a match arm body")
	p.advance()
	p.skipNewlinesAfterAssign()
	p.expr()
}

func (p *parser) nestedStatic() NodeID {
	p.errorHelp(p.tokSpan(p.pos), "declare the `static` at the top level of a file; inside a function use `let`", "a `static` can only be declared at the top level")
	p.staticDecl(prefix{})
	return 0
}
