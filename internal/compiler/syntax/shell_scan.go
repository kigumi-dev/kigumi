package syntax

// shellStringEnd scans `$"..."` from its opening quote at pos and returns the offset past the closing quote. A raw `${`
// interpolates only where the shell parser reads one: not in single quotes, nor after a backslash outside quotes.
func shellStringEnd(src []byte, pos, end int, escape func(int) int, interp func(int) (int, bool)) (int, bool) {
	q := ShellBare
	interpAt := func(p int) bool { return p+1 < end && src[p] == '$' && src[p+1] == '{' }
	step := func(p int) int {
		if src[p] == '\\' {
			return max(escape(p), p+1)
		}
		return p + 1
	}
	for pos++; pos < end; {
		scanOps.Add(1)
		if src[pos] == '"' {
			return pos + 1, true
		}
		if q != ShellSingle && interpAt(pos) {
			var ok bool
			if pos, ok = interp(pos + 2); !ok {
				return pos, false
			}
			continue
		}
		c, _, _ := shellCharAt(src, pos, end)
		pos = step(pos)
		switch {
		case c == '\'' && q == ShellBare:
			q = ShellSingle
		case c == '\'' && q == ShellSingle, c == '"' && q == ShellDouble:
			q = ShellBare
		case c == '"' && q == ShellBare:
			q = ShellDouble
		case c == '\\' && q != ShellSingle && pos < end && src[pos] != '"' && !(q == ShellDouble && interpAt(pos)):
			pos = step(pos)
		}
	}
	return pos, false
}

// shellQuoteAt reports whether the quote at pos opens a shell literal: a `$` before it on the same line.
func shellQuoteAt(src []byte, pos int) bool {
	for i := pos - 1; i >= 0; i-- {
		switch src[i] {
		case ' ', '\t':
			continue
		case '$':
			return true
		}
		return false
	}
	return false
}

// skipShellString is shellStringEnd for the parser's brace matching, which reports no escape errors.
func skipShellString(src []byte, pos, end int, out map[int]interpInfo) int {
	escape := func(p int) int {
		_, n, _ := shellCharAt(src, p, end)
		return p + n
	}
	interp := func(p int) (int, bool) {
		close := matchBrace(src, p, end, out)
		return min(close+1, end), close < end
	}
	pos, _ = shellStringEnd(src, pos, end, escape, interp)
	return pos
}
