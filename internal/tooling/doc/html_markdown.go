package doc

import (
	"html"
	"strconv"
	"strings"
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
