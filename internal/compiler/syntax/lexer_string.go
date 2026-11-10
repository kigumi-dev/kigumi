package syntax

import (
	"strconv"
	"unicode/utf8"

	"kigumi/internal/lang/token"
)

func (lx *lexer) lexString() {
	start := lx.pos
	_, hinted := lx.hints[start]
	var end int
	var ok bool
	if n := len(lx.toks); n > 0 && lx.toks[n-1].Kind == token.Dollar {
		end, ok = shellStringEnd(lx.src, lx.pos, lx.end, lx.scanEscape, lx.scanInterpolation)
		hinted = false
	} else {
		end, ok = lx.scanString(lx.pos)
	}
	lx.pos = min(end, lx.end)
	// The outermost lex already checked a hinted string's UTF-8; rechecking per nesting level is the O(n²) walk hints avoid.
	if !ok {
		lx.errorf(start, lx.pos, "unterminated string literal")
	} else if !hinted {
		utf8ValidBytes.Add(int64(lx.pos - start))
		if !utf8.Valid(lx.src[start:lx.pos]) {
			lx.errorf(start, lx.pos, "string literal contains invalid UTF-8")
		}
	}
	lx.emit(token.String, start, lx.pos)
}

// lx.hints skip to a known end: subExpr lexes each `${...}` body afresh, so without them a string nested d levels deep
// would be walked d times.
func (lx *lexer) scanString(pos int) (int, bool) {
	if info, ok := lx.hints[pos]; ok {
		return info.end, info.closed
	}
	pos++
	for pos < lx.end {
		scanOps.Add(1)
		switch lx.src[pos] {
		case '\\':
			pos = lx.scanEscape(pos)
		case '"':
			return pos + 1, true
		case '$':
			if pos+1 < lx.end && lx.src[pos+1] == '{' {
				var ok bool
				pos, ok = lx.scanInterpolation(pos + 2)
				if !ok {
					return pos, false
				}
				continue
			}
			pos++
		default:
			pos++
		}
	}
	return pos, false
}

func (lx *lexer) scanInterpolation(pos int) (int, bool) {
	depth := 1
	for pos < lx.end {
		switch lx.src[pos] {
		case '{':
			depth++
			pos++
		case '}':
			depth--
			pos++
			if depth == 0 {
				return pos, true
			}
		case '"':
			var ok bool
			if lx.byteStringPrefixAt(pos) {
				pos, ok = lx.scanByteString(pos)
			} else if shellQuoteAt(lx.src, pos) {
				pos, ok = shellStringEnd(lx.src, pos, lx.end, lx.scanEscape, lx.scanInterpolation)
			} else {
				pos, ok = lx.scanString(pos)
			}
			if !ok {
				return pos, false
			}
		case '\'':
			if n := charLiteralLen(lx.src, pos, lx.end); n > 0 {
				pos += n
				continue
			}
			pos++
		case '#':
			pos = interpLineCommentEnd(lx.src, pos+1, lx.end, depth)
			continue
		case '/':
			if pos+1 < lx.end && lx.src[pos+1] == '/' {
				pos = interpLineCommentEnd(lx.src, pos+2, lx.end, depth)
				continue
			}
			if pos+1 < lx.end && lx.src[pos+1] == '*' {
				var closed bool
				if pos, closed = blockCommentEnd(lx.src, pos, lx.end); !closed {
					return pos, false
				}
				continue
			}
			pos++
		default:
			pos++
		}
	}
	lx.errorf(pos, pos, "unterminated interpolation")
	return pos, false
}

// byteStringPrefixAt mirrors the top-level `b"..."` rule in lexer.go's step.
func (lx *lexer) byteStringPrefixAt(pos int) bool {
	if pos < 1 || lx.src[pos-1] != 'b' {
		return false
	}
	return pos < 2 || !isIdentContinueByte(lx.src[pos-2])
}

func (lx *lexer) scanEscape(pos int) int {
	start := pos
	pos++
	if pos >= lx.end {
		return pos
	}
	switch lx.src[pos] {
	case 'n', 't', 'r', '0', '\\', '"', '\'', '$':
		return pos + 1
	case 'u':
		if pos+1 < lx.end && lx.src[pos+1] == '{' {
			digStart := pos + 2
			pos = digStart
			for pos < lx.end && lx.src[pos] != '}' && lx.src[pos] != '"' {
				pos++
			}
			if pos < lx.end && lx.src[pos] == '}' && pos > digStart {
				if v, err := strconv.ParseUint(string(lx.src[digStart:pos]), 16, 32); err == nil && utf8.ValidRune(rune(v)) {
					return pos + 1
				}
			}
			// The brace never closed before the quote: leave the quote for the
			// string scanner to end the literal instead of swallowing the file.
			if pos < lx.end && lx.src[pos] == '"' {
				lx.errorf(start, pos, "invalid escape sequence")
				return pos
			}
		}
	}
	lx.errorf(start, min(pos+1, lx.end), "invalid escape sequence")
	return min(pos+1, lx.end)
}

// lexChar lexes `'x'` and also, since Rust-style lifetimes share the quote
// with char literals, `'a` (a lifetime): an identifier after `'` not itself
// closed by another `'` is a lifetime, matching how rustc disambiguates them.
func (lx *lexer) lexChar() {
	start := lx.pos
	if p := lx.pos + 1; p < lx.end && isIdentStart(lx.src[p]) {
		end := p
		for end < lx.end && isIdentContinueByte(lx.src[end]) {
			end++
		}
		if end >= lx.end || lx.src[end] != '\'' {
			lx.pos = end
			lx.emit(token.Lifetime, start, lx.pos)
			return
		}
	}
	pos := lx.pos + 1
	switch {
	case pos >= lx.end:
	case lx.src[pos] == '\\':
		pos = lx.scanEscape(pos)
	case lx.src[pos] == '\'' || lx.src[pos] == '\n':
	default:
		_, size := utf8.DecodeRune(lx.src[pos:lx.end])
		pos += size
	}
	if pos < lx.end && lx.src[pos] == '\'' {
		lx.pos = pos + 1
		lx.emit(token.Char, start, lx.pos)
		return
	}
	lx.pos = pos
	lx.errorf(start, lx.pos, "invalid character literal")
	lx.emit(token.Char, start, lx.pos)
}
