package doc

import (
	"strconv"
	"strings"
	"unicode"
)

// trimTOC drops third-level entries when a package has enough symbols that the
// TOC would become a second copy of the page. Second-level sections stay, so
// the TOC keeps overview entries for large pages.
func trimTOC(entries []tocEntry) []tocEntry {
	h2, h3 := 0, 0
	for _, e := range entries {
		switch e.Level {
		case 2:
			h2++
		case 3:
			h3++
		}
	}
	if h2 == 0 || h3 <= 40 {
		return entries
	}
	out := make([]tocEntry, 0, h2)
	for _, e := range entries {
		if e.Level == 2 {
			out = append(out, e)
		}
	}
	return out
}

// plainInline strips the inline forms that would be wrong inside a TOC link or
// an aria-label: `code` keeps its text, [label](target) keeps the label.
func plainInline(text string) string {
	var sb strings.Builder
	for len(text) > 0 {
		switch {
		case text[0] == '`':
			end := strings.IndexByte(text[1:], '`')
			if end < 0 {
				sb.WriteString(text)
				return sb.String()
			}
			sb.WriteString(text[1 : 1+end])
			text = text[end+2:]
		case text[0] == '[':
			mid := strings.Index(text, "](")
			end := strings.IndexByte(text, ')')
			if mid < 0 || end < mid {
				sb.WriteByte(text[0])
				text = text[1:]
				continue
			}
			sb.WriteString(text[1:mid])
			text = text[end+1:]
		default:
			i := strings.IndexAny(text, "`[")
			if i < 0 {
				i = len(text)
			}
			sb.WriteString(text[:i])
			text = text[i:]
		}
	}
	return sb.String()
}

// uniqueAnchor dedups a base id like GitHub's renderer ("-2", "-3", ...) and skips ids already
// taken; seen holds both the bases and the ids handed out.
func uniqueAnchor(seen map[string]int, base string) string {
	n := seen[base]
	id := base
	if n > 0 {
		id = base + "-" + strconv.Itoa(n+1)
		for seen[id] > 0 {
			n++
			id = base + "-" + strconv.Itoa(n+1)
		}
	}
	seen[base] = n + 1
	if id != base {
		seen[id]++
	}
	return id
}

func anchor(text string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '.' || r == '_' || r == '-':
			sb.WriteRune(r)
		case r == ' ':
			sb.WriteByte('-')
		}
	}
	return sb.String()
}
