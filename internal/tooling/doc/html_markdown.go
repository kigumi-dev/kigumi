package doc

import (
	"html"
	"strconv"
	"strings"
)

// markdownToHTML converts the subset of Markdown the renderer emits, resolving
// hrefs relative to selfDir and uniquing heading ids for overloaded names.
func markdownToHTML(md, selfDir string) string {
	var sb strings.Builder
	lines := strings.Split(md, "\n")
	inCode, inList, inPara := false, false, false
	var codeLang string
	var codeLines []string
	seen := map[string]int{}
	closePara := func() {
		if inPara {
			sb.WriteString("</p>\n")
			inPara = false
		}
	}
	closeList := func() {
		if inList {
			sb.WriteString("</ul>\n")
			inList = false
		}
	}
	flushCode := func() {
		writeCodeBody(&sb, codeLang, strings.Join(codeLines, "\n"))
		sb.WriteString("</code></pre>\n")
		codeLines = nil
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "```"):
			closePara()
			closeList()
			if inCode {
				flushCode()
			} else {
				codeLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				sb.WriteString("<pre><code" + langClass(codeLang) + ">")
			}
			inCode = !inCode
		case inCode:
			codeLines = append(codeLines, line)
		case strings.HasPrefix(line, "#"):
			closePara()
			closeList()
			level := len(line) - len(strings.TrimLeft(line, "#"))
			text := strings.TrimSpace(line[level:])
			id := uniqueAnchor(seen, anchor(text))
			tag := "h" + strconv.Itoa(level)
			sb.WriteString("<" + tag + " id=\"" + html.EscapeString(id) + "\">" + inline(text, selfDir) + "</" + tag + ">\n")
		case strings.HasPrefix(line, "- "):
			closePara()
			if !inList {
				sb.WriteString("<ul>\n")
				inList = true
			}
			sb.WriteString("<li>" + inline(strings.TrimPrefix(line, "- "), selfDir) + "</li>\n")
		case strings.TrimSpace(line) == "":
			closePara()
			closeList()
		default:
			closeList()
			if !inPara {
				sb.WriteString("<p>")
				inPara = true
			} else {
				sb.WriteString(" ")
			}
			sb.WriteString(inline(line, selfDir))
		}
	}
	closePara()
	closeList()
	if inCode {
		flushCode()
	}
	return sb.String()
}

func langClass(lang string) string {
	if lang == "" {
		return ""
	}
	return " class=\"language-" + html.EscapeString(lang) + "\""
}

func writeCodeBody(sb *strings.Builder, lang, body string) {
	if body != "" {
		body += "\n"
	}
	if lang == "kigumi" {
		sb.WriteString(highlightKigumi(body))
	} else {
		sb.WriteString(html.EscapeString(body))
	}
}

// uniqueAnchor dedups a base id the way GitHub's Markdown renderer does: the
// first occurrence keeps the bare id, later ones get "-2", "-3", ...
func uniqueAnchor(seen map[string]int, base string) string {
	n := seen[base]
	seen[base] = n + 1
	if n == 0 {
		return base
	}
	return base + "-" + strconv.Itoa(n+1)
}

func anchor(text string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_':
			sb.WriteRune(r)
		case r == ' ' || r == '-':
			sb.WriteByte('-')
		}
	}
	return sb.String()
}
