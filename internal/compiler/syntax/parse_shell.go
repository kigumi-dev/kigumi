package syntax

import (
	"kigumi/internal/lang/token"
)

// shellLit parses `$"..."` into a ShellPlan, reading the source a language character at a time (shellUnitAt) so the
// pieces keep their raw escapes for DecodeShellText.
func (p *parser) shellLit() NodeID {
	dollar := p.expect(token.Dollar)
	if !p.at(token.String) {
		p.errorf("expected string literal after `$`")
		return 0
	}
	if p.toks[p.pos-1].Kind == token.Newline || p.tree.File.Line(p.tok().Start) != p.tree.File.Line(p.toks[dollar].Start) {
		p.errorf("`$` and its string must be on the same line")
	}
	str := p.advance()
	t := p.toks[str]
	sh := &shellParser{p: p, src: p.tree.File.Src, pos: int(t.Start) + 1, end: int(t.End) - 1}
	plan := sh.plan(str)
	return p.tree.add(rawNode{Kind: ShellLit, Tok: dollar, Lhs: str, Rhs: uint32(plan)})
}

type shellParser struct {
	p        *parser
	src      []byte
	pos, end int
}

func (s *shellParser) errorf(start int, msg string) {
	end := max(s.pos, start+1)
	s.p.bag.Add(syntaxError(token.Span{Start: token.Pos(start), End: token.Pos(end)}, msg))
}

func (s *shellParser) plan(tok uint32) NodeID {
	var cmds []NodeID
	for {
		cmds = append(cmds, s.command(tok))
		s.skipSpace()
		c, n, _ := s.at(s.pos)
		if c != '|' {
			break
		}
		if next, m, _ := s.at(s.pos + n); next == '|' {
			s.errorf(s.pos, "`||` is not part of the portable shell dialect")
			n += m
		}
		s.pos += n
	}
	if s.pos < s.end {
		s.errorf(s.pos, "unsupported shell syntax")
	}
	return s.p.tree.addList(ShellPlan, tok, cmds)
}

func (s *shellParser) skipSpace() {
	for c, n, _ := s.at(s.pos); c == ' ' || c == '\t' || c == '\n'; c, n, _ = s.at(s.pos) {
		s.pos += n
	}
}

func (s *shellParser) command(tok uint32) NodeID {
	var parts []NodeID
	for {
		s.skipSpace()
		if s.pos >= s.end {
			break
		}
		c, n, _ := s.at(s.pos)
		next, m, _ := s.at(s.pos + n)
		switch {
		case c == '|':
			return s.p.tree.addList(ShellCmd, tok, parts)
		case c == '&' && next == '&':
			s.errorf(s.pos, "`&&` is not part of the portable shell dialect")
			s.pos += n + m
		case c == '&' || c == ';' || c == '(' || c == ')' || c == '`':
			s.errorf(s.pos, "`"+string(c)+"` is not part of the portable shell dialect")
			s.pos += n
		case c == '<' || c == '>' || isDigitRune(c) && s.isRedirectAfterDigits():
			parts = append(parts, s.redirect(tok))
		default:
			before := s.pos
			parts = append(parts, s.word(tok))
			if s.pos == before {
				s.errorf(s.pos, "unexpected character in shell literal")
				s.pos += n
			}
		}
	}
	return s.p.tree.addList(ShellCmd, tok, parts)
}

func (s *shellParser) isRedirectAfterDigits() bool {
	i := s.pos
	c, n, _ := s.at(i)
	for ; isDigitRune(c); c, n, _ = s.at(i) {
		i += n
	}
	return c == '<' || c == '>'
}

// maxRedirectFd is far above any file descriptor a real process can have
// open; digits past it stop accumulating so the uint32 fd field can never
// wrap around.
const maxRedirectFd = 1_000_000

func (s *shellParser) redirect(tok uint32) NodeID {
	start := s.pos
	fd := uint32(0)
	hasFd := false
	tooBig := false
	c, n, _ := s.at(s.pos)
	for ; isDigitRune(c); c, n, _ = s.at(s.pos) {
		if fd > maxRedirectFd {
			tooBig = true
		} else {
			fd = fd*10 + uint32(c-'0')
		}
		s.pos += n
		hasFd = true
	}
	if tooBig {
		s.errorf(start, sprintf("redirect file descriptor is too large (max is %d)", maxRedirectFd))
	}
	op := RedirIn
	if c == '>' {
		op = RedirOut
		if !hasFd {
			fd = 1
		}
	}
	s.pos += n
	if c, n, _ = s.at(s.pos); c == '>' && op == RedirOut {
		op = RedirAppend
		s.pos += n
	} else if c == '<' && op == RedirIn {
		s.errorf(start, "here documents are not supported")
		s.pos += n
	}
	if c, n, _ = s.at(s.pos); c == '&' {
		s.pos += n
		if op == RedirIn {
			op = RedirDupIn
		} else {
			op = RedirDupOut
		}
	}
	s.skipSpace()
	if c, n, _ = s.at(s.pos); n == 0 || c == '|' {
		s.errorf(start, "redirect needs a target")
		return s.p.tree.addRec(ShellRedirect, tok, []uint32{op, fd, 0})
	}
	target := s.word(tok)
	return s.p.tree.addRec(ShellRedirect, tok, []uint32{op, fd, uint32(target)})
}
