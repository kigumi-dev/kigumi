package syntax

// word parses one argv element, stopping at unquoted whitespace or a shell operator.
func (s *shellParser) word(tok uint32) NodeID {
	var parts []NodeID
	if c, _, _ := s.at(s.pos); c == '#' {
		s.errorf(s.pos, "a `#` comment is not part of the portable shell dialect")
		s.pos = s.end
		return s.p.tree.addList(ShellWord, tok, parts)
	}
	for s.pos < s.end {
		c, _, esc := s.at(s.pos)
		switch {
		case isShellBreak(c):
			return s.p.tree.addList(ShellWord, tok, parts)
		case c == '"':
			parts = append(parts, s.doubleQuoted(tok)...)
		case c == '\'':
			parts = append(parts, s.singleQuoted())
		case !esc && s.interpAt(s.pos):
			parts = append(parts, s.interpolation(tok))
		case !esc && c == '$':
			s.errorf(s.pos, "`$name` expansion is not allowed; use `${name}`")
			s.pos++
		default:
			parts = append(parts, s.bareText())
		}
	}
	return s.p.tree.addList(ShellWord, tok, parts)
}

func (s *shellParser) interpAt(pos int) bool {
	return pos+1 < s.end && s.src[pos] == '$' && s.src[pos+1] == '{'
}

func (s *shellParser) text(start, end int, q ShellQuote) NodeID {
	return s.p.tree.addRec(ShellText, 0, []uint32{uint32(start), uint32(end), uint32(q)})
}

func (s *shellParser) skipChar() {
	_, n, _ := s.at(s.pos)
	s.pos += n
}

// bareText reads unquoted text; a backslash escapes the next character, the `$` of an interpolation included, as
// POSIX does for `\$`.
func (s *shellParser) bareText() NodeID {
	start := s.pos
	for s.pos < s.end {
		c, n, esc := s.at(s.pos)
		if isShellBreak(c) || c == '\'' || c == '"' || !esc && c == '$' {
			break
		}
		s.pos += n
		if c == '\\' {
			s.skipChar()
		}
	}
	return s.text(start, s.pos, ShellBare)
}

func (s *shellParser) singleQuoted() NodeID {
	start := s.pos
	s.skipChar()
	seg := s.pos
	for s.pos < s.end {
		c, n, _ := s.at(s.pos)
		if c == '\'' {
			t := s.text(seg, s.pos, ShellSingle)
			s.pos += n
			return t
		}
		s.pos += n
	}
	s.errorf(start, "unterminated single quote in shell literal")
	return s.text(seg, s.end, ShellSingle)
}

// doubleQuoted reads a double-quoted segment; a backslash escapes the next character for the scan, except that an
// interpolation after it still interpolates.
func (s *shellParser) doubleQuoted(tok uint32) []NodeID {
	var parts []NodeID
	start := s.pos
	s.skipChar()
	seg := s.pos
	flush := func() {
		if s.pos > seg {
			parts = append(parts, s.text(seg, s.pos, ShellDouble))
		}
	}
	for s.pos < s.end {
		c, n, esc := s.at(s.pos)
		switch {
		case c == '"':
			flush()
			s.pos += n
			return parts
		case !esc && s.interpAt(s.pos):
			flush()
			parts = append(parts, s.interpolation(tok))
			seg = s.pos
		default:
			s.pos += n
			if c == '\\' && !s.interpAt(s.pos) {
				s.skipChar()
			}
		}
	}
	s.errorf(start, "unterminated double quote in shell literal")
	return parts
}

func (s *shellParser) interpolation(tok uint32) NodeID {
	close := matchBrace(s.src, s.pos+2, s.end, nil)
	e := s.p.subExpr(s.pos+2, close)
	s.pos = min(close+1, s.end)
	return s.p.node1(ShellInterp, tok, e)
}
