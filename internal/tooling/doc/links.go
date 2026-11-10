package doc

import "strings"

// mdLink is one inline Markdown link found in text.
type mdLink struct {
	label string
	dest  string
	end   int // offset just past the closing ')'
}

// scanLink parses the inline link that starts at text[at] == '['. Labels may
// nest brackets; destinations may contain balanced parentheses and backslash
// escapes, and an angle-bracketed destination ends at its '>'. It reports false
// when the text does not hold a complete link, so the caller can fall back to
// treating the bracket as literal. Both the href builder and the site
// validators go through this parser, so a target is cut at the same place
// everywhere.
func scanLink(text string, at int) (mdLink, bool) {
	if at < 0 || at >= len(text) || text[at] != '[' {
		return mdLink{}, false
	}
	close := matchingBracket(text, at)
	if close < 0 || close+1 >= len(text) || text[close+1] != '(' {
		return mdLink{}, false
	}
	dest, end, ok := scanDest(text, close+2)
	if !ok {
		return mdLink{}, false
	}
	return mdLink{label: unescapeDest(text[at+1 : close]), dest: dest, end: end}, true
}

// matchingBracket returns the index of the ']' closing the '[' at open, or -1
// when it never closes. Backslash escapes and nested brackets count the way
// CommonMark reads a link label.
func matchingBracket(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// scanDest reads the destination of a link whose '(' ends at at, returning it
// plus the offset one past the closing ')'. Parentheses inside the destination
// must balance, as CommonMark allows; a space ends the destination and starts
// an optional title, which is consumed up to the ')'.
func scanDest(text string, at int) (string, int, bool) {
	i := at
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	if i < len(text) && text[i] == '<' {
		for j := i + 1; j < len(text); j++ {
			switch text[j] {
			case '\\':
				j++
			case '>':
				if k := closingParen(text, j+1); k >= 0 {
					return unescapeDest(text[i+1 : j]), k + 1, true
				}
				return "", 0, false
			}
		}
		return "", 0, false
	}
	depth, start := 0, i
	for ; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return unescapeDest(text[start:i]), i + 1, true
			}
			depth--
		case ' ', '\t', '\n':
			if k := closingParen(text, i); k >= 0 {
				return unescapeDest(text[start:i]), k + 1, true
			}
			return "", 0, false
		}
	}
	return "", 0, false
}

// closingParen returns the index of the ')' that ends a link once its
// destination is complete, or -1 when the link runs past the line.
func closingParen(text string, at int) int {
	for i := at; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case ')':
			return i
		case '\n':
			return -1
		}
	}
	return -1
}

// linkTargets returns the destination of every inline link in src in order.
func linkTargets(src string) []string {
	var out []string
	for i := 0; i < len(src); {
		if src[i] != '[' {
			i++
			continue
		}
		l, ok := scanLink(src, i)
		if !ok {
			i++
			continue
		}
		out = append(out, l.dest)
		i = l.end
	}
	return out
}

// unescapeDest resolves the backslash escapes CommonMark allows in a link
// label or destination, so `\(` reaches the href as `(`.
func unescapeDest(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]) {
			i++
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}
