package doc

import (
	"html"
	"strconv"
	"strings"
	"unicode"
)

// markdownPage converts the subset of Markdown the renderer emits into HTML,
// resolving hrefs relative to selfDir and uniquing heading ids. It also returns
// the second- and third-level headings for the on-page table of contents.
func markdownPage(md, selfDir string) (string, []tocEntry) {
	var sb strings.Builder
	lines := strings.Split(md, "\n")
	inCode, inList, inPara := false, false, false
	var codeLang string
	var codeLines []string
	var toc []tocEntry
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
		sb.WriteString("</code></pre>\n</div>\n")
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
				sb.WriteString(`<div class="code-block" data-lang="` + html.EscapeString(codeLang) + `"><pre><code` + langClass(codeLang) + `>`)
			}
			inCode = !inCode
		case inCode:
			codeLines = append(codeLines, line)
		case strings.HasPrefix(line, "#"):
			closePara()
			closeList()
			level := len(line) - len(strings.TrimLeft(line, "#"))
			text := strings.TrimSpace(line[level:])
			base := anchor(text)
			if base == "" {
				base = "section"
			}
			id := uniqueAnchor(seen, base)
			if level >= 2 && level <= 3 {
				toc = append(toc, tocEntry{ID: id, Text: plainInline(text), Level: level})
			}
			tag := "h" + strconv.Itoa(level)
			sb.WriteString("<" + tag + " id=\"" + html.EscapeString(id) + "\">")
			if level >= 2 && level <= 4 {
				sb.WriteString(`<a class="heading-anchor" href="#` + html.EscapeString(id) + `" aria-label="Permalink: ` + html.EscapeString(plainInline(text)) + `" tabindex="-1">#</a>`)
			}
			sb.WriteString(inline(text, selfDir) + "</" + tag + ">\n")
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
	toc = trimTOC(toc)
	return sb.String(), toc
}

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

// markdownToHTML is markdownPage without the TOC data, kept for callers that
// only need the rendered body.
func markdownToHTML(md, selfDir string) string {
	body, _ := markdownPage(md, selfDir)
	return body
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
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-':
			sb.WriteRune(r)
		case r == ' ':
			sb.WriteByte('-')
		}
	}
	return sb.String()
}
