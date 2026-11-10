package syntax

import (
	"bytes"
	"strings"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// Incomplete reports whether src is a syntactically unfinished fragment whose
// user most likely intends to keep typing, and if so a short reason such as
// "unclosed `(`". A REPL uses it to decide whether to show a continuation
// prompt instead of parsing and reporting an error.
//
// Detection is lexical: it never builds a tree. The fragment is unfinished
// when any of these hold:
//
//   - a string, byte string, interpolation, character literal, or block
//     comment is unterminated;
//   - a `(`, `[`, or `{` is still open at end of input;
//   - the last significant token is a binary or prefix operator, `=`,
//     assignment operator, `,`, `:`, `.`, `..`, `?.`, `=>`, `->`, `@`, `$`,
//     or a keyword that cannot end a construct (for example `let`, `match`,
//     `else`);
//   - the last line starts a `let`/`const`/`type` declaration that has not
//     reached its `=` (or, for `type`, its body).
//
// Mismatched closers and other malformed input are reported as complete so
// the parser can produce a real diagnostic rather than a continuation prompt.
// Complete valid input always yields false, including braces and operators
// that appear inside string literals, `${...}` interpolation, and comments.
func Incomplete(src []byte) (bool, string) {
	var bag diag.Bag
	toks := Lex(src, &bag)
	// The lexer marks every spelling of "ran out of input" as "unterminated".
	// Interpolation can make an unterminated string token end in `"`, so the
	// token text cannot be inspected alone.
	for _, d := range bag.Sorted() {
		if strings.HasPrefix(d.Msg, "unterminated") {
			return true, d.Msg
		}
	}

	var stack []token.Kind
	var last token.Token
	haveLast := false
	// First significant token on the current line, used for declaration starts.
	var lineFirst token.Token
	haveLineFirst := false
	lineHasAssign := false
	lineHasBrace := false

	for _, t := range toks {
		switch {
		case t.Kind == token.BOF, t.Kind == token.EOF:
			continue
		case t.Kind == token.Newline:
			haveLineFirst = false
			lineHasAssign = false
			lineHasBrace = false
			continue
		case t.Kind.IsTrivia():
			continue
		}
		if !haveLineFirst {
			lineFirst = t
			haveLineFirst = true
		}
		switch t.Kind {
		case token.Assign:
			lineHasAssign = true
		case token.LBrace, token.RBrace:
			lineHasBrace = true
		}
		// Literals are single tokens, so delimiters inside a string or a
		// comment never reach this stack.
		switch t.Kind {
		case token.LParen, token.LBracket, token.LBrace:
			stack = append(stack, t.Kind)
		case token.RParen, token.RBracket, token.RBrace:
			want, _ := openerOf(t.Kind)
			if len(stack) == 0 || stack[len(stack)-1] != want {
				return false, ""
			}
			stack = stack[:len(stack)-1]
		}
		last = t
		haveLast = true
	}
	if len(stack) > 0 {
		return true, "unclosed `" + stack[len(stack)-1].String() + "`"
	}
	if !haveLast {
		return false, ""
	}
	// A character literal that ran into the end of input is lexed as Char
	// without its closing quote; `'a` at end of input is instead a Lifetime,
	// which can never end a complete construct either.
	if last.Kind == token.Char && !bytes.HasSuffix(src[last.Start:last.End], []byte("'")) {
		return true, "unterminated character literal"
	}
	if last.Kind == token.Lifetime {
		return true, "unterminated character literal or missing type after the lifetime"
	}
	if reason, ok := unfinishedAfter(last, src); ok {
		return true, reason
	}
	// A declaration can end on a bare pattern or type; require its `=`.
	if haveLineFirst && !lineHasAssign && !lineHasBrace {
		switch lineFirst.Kind {
		case token.KwLet, token.KwConst:
			return true, "expected `=` and a value after the declaration"
		case token.KwType:
			return true, "expected `=` or a body after the type declaration"
		}
	}
	return false, ""
}

// unfinishedAfter reports whether t cannot be the last token of a complete
// construct. The returned reason names the token so the REPL can prompt for
// exactly what is expected next.
func unfinishedAfter(t token.Token, src []byte) (string, bool) {
	quoted := "`" + t.Text(src) + "`"
	switch t.Kind {
	case token.Assign:
		return "expected a value after `=`", true
	case token.PlusAssign, token.MinusAssign, token.StarAssign, token.SlashAssign,
		token.PercentAssign, token.AmpAssign, token.PipeAssign, token.CaretAssign,
		token.ShlAssign, token.ShrAssign:
		return "expected a value after `" + t.Kind.String() + "`", true
	case token.Comma:
		return "expected another item after `,`", true
	case token.Colon:
		return "expected a type or value after `:`", true
	case token.Dot, token.DotDot, token.DotDotEq, token.Ellipsis, token.QuestionDot,
		token.Arrow, token.FatArrow, token.At, token.Dollar:
		return "expected more after " + quoted, true
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
		token.Amp, token.Pipe, token.Caret, token.Shl, token.Shr,
		token.EqEq, token.NotEq, token.Lt, token.LtEq, token.Gt, token.GtEq,
		token.AndAnd, token.OrOr, token.PipeGt, token.Operator, token.Tilde:
		return "expected an operand after " + quoted, true
	}
	switch t.Kind {
	case token.KwFn, token.KwLet, token.KwConst, token.KwMut, token.KwType,
		token.KwInterface, token.KwPub, token.KwImport, token.KwIf, token.KwElse,
		token.KwMatch, token.KwIs, token.KwFor, token.KwIn, token.KwDefer,
		token.KwPure, token.KwNoalloc, token.KwAsync, token.KwAwait,
		token.KwUnsafe, token.KwMove, token.KwExtern, token.KwComptime:
		return "expected more after " + quoted, true
	}
	return "", false
}
