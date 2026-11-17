package diag

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/lang/token"
)

// Render formats one diagnostic in the rustc-familiar layout the golden tests pin; notes with a location follow as their own note: snippet.
func Render(src Sources, d Diagnostic) string {
	var sb strings.Builder
	renderInto(&sb, src, d)
	return sb.String()
}

func RenderAll(src Sources, ds []Diagnostic) string {
	var sb strings.Builder
	for _, d := range ds {
		renderInto(&sb, src, d)
	}
	return sb.String()
}

func renderInto(sb *strings.Builder, src Sources, d Diagnostic) {
	f := source(src, d.Loc.Source)
	width := gutterWidth(src, d)
	renderHead(sb, d)
	if f != nil {
		renderSnippet(sb, f, d.Loc.Span, d.Severity, width)
	}
	for _, h := range d.Help {
		fmt.Fprintf(sb, "%s = %s %s\n", strings.Repeat(" ", width), paint(ansiCyan, "help:"), escapeC0(h))
	}
	for _, n := range d.Notes {
		renderHead(sb, n)
		if nf := source(src, n.Loc.Source); nf != nil {
			renderSnippet(sb, nf, n.Loc.Span, n.Severity, width)
		}
	}
	sb.WriteByte('\n')
}

// source resolves a location, treating a nil Sources as one with no files so a
// diagnostic with no source location (a release-policy finding) still renders.
func source(src Sources, id token.SourceID) *token.File {
	if src == nil {
		return nil
	}
	return src.Source(id)
}

func renderHead(sb *strings.Builder, d Diagnostic) {
	word := d.Severity.String()
	if d.Code != "" {
		word += "[" + d.Code + "]"
	}
	fmt.Fprintf(sb, "%s%s\n", paint(severityColor(d.Severity), word), paint(ansiBold, ": "+escapeC0(d.Msg)))
}

// renderSnippet positions the underline in display cells, not byte columns, since CJK/emoji/tab bytes vary in width.
func renderSnippet(sb *strings.Builder, f *token.File, span token.Span, sev Severity, width int) {
	line, col := f.Line(span.Start), f.Column(span.Start)
	pad := strings.Repeat(" ", width)
	fmt.Fprintf(sb, "%s%s %s:%d:%d\n", pad, paint(ansiBlue, "-->"), f.Name, line, col)
	fmt.Fprintf(sb, "%s %s\n", pad, paint(ansiBlue, "|"))
	num := strconv.Itoa(line)
	lineStart := f.LineSpan(line).Start
	lineText, leadCells, cells := snippetLine(f.LineText(line), int(span.Start-lineStart), int(span.End-lineStart))
	fmt.Fprintf(sb, "%s%s %s %s\n", strings.Repeat(" ", width-len(num)), paint(ansiBlue, num), paint(ansiBlue, "|"), lineText)
	carets := strings.Repeat("^", cells)
	fmt.Fprintf(sb, "%s %s %s%s\n", pad, paint(ansiBlue, "|"), strings.Repeat(" ", leadCells), paint(underlineColor(sev), carets))
}

// gutterWidth is the width of the widest line number the diagnostic and
// its notes show, so their gutters line up.
func gutterWidth(src Sources, d Diagnostic) int {
	width := 1
	consider := func(loc Location) {
		if f := source(src, loc.Source); f != nil {
			width = max(width, len(strconv.Itoa(f.Line(loc.Span.Start))))
		}
	}
	consider(d.Loc)
	for _, n := range d.Notes {
		consider(n.Loc)
	}
	return width
}

// escapeC0 keeps control bytes out of the terminal; tabs and newlines are
// already meaningful in source snippets and stay as they are.
func escapeC0(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 && c != '\n' && c != '\t' || c == 0x7f {
			fmt.Fprintf(&out, "\\x%02x", c)
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}
