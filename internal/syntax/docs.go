package syntax

import (
	"strings"

	"kigumi/internal/token"
)

// DeclHead is the source of a declaration without its body (functions)
// or with it (types, interfaces, consts), capped at maxHeadLines lines.
func DeclHead(t *Tree, n NodeID) string {
	_, lines := declSource(t, n)
	return capHead(lines)
}

// declSource is the declaration's source lines and the 1-based number of the first one; nil for a node that is not a declaration.
func declSource(t *Tree, n NodeID) (int, []string) {
	switch t.Kind(n) {
	case FnDecl, TypeDecl, InterfaceDecl, ConstDecl, StaticDecl:
	default:
		return 0, nil
	}
	sp := t.Span(n)
	if body := declBody(t, n); body.Valid() {
		sp.End = body.Span().Start
	} else {
		// A node span stops at its last child, so a trailing `]` or `}`
		// can fall outside it; extend to the line that balances an open brace.
		sp.End = t.File.LineSpan(t.File.Line(sp.End - 1)).End
		for depth(t.File.Src[sp.Start:sp.End]) > 0 && int(sp.End) < len(t.File.Src) {
			sp.End = t.File.LineSpan(t.File.Line(sp.End) + 0).End
			if int(sp.End) < len(t.File.Src) && t.File.Src[sp.End] == '\n' {
				sp.End++
			}
		}
	}
	text := string(t.File.Src[sp.Start:sp.End])
	first := t.File.Line(sp.Start) + strings.Count(text[:len(text)-len(strings.TrimLeft(text, " \t\r\n"))], "\n")
	return first, strings.Split(strings.TrimSpace(text), "\n")
}

func capHead(lines []string) string {
	for len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "///") {
		lines = lines[1:]
	}
	if len(lines) > maxHeadLines {
		lines = append(lines[:maxHeadLines], "    ...")
	}
	return strings.Join(lines, "\n")
}

const maxHeadLines = 40

func declBody(t *Tree, n NodeID) BlockView {
	if t.Kind(n) != FnDecl {
		return BlockView{}
	}
	return t.View(n).AsFnDecl().Body()
}

// DocText joins the `///` lines attached to a declaration; the parser keeps
// only the last one in the doc slot, so the earlier lines are read back
// from the token stream.
func DocText(t *Tree, n NodeID) string {
	last := t.View(n).AsDecl().Doc().Index()
	if last == 0 {
		return ""
	}
	var lines []string
	for i := FirstDocTok(t, last); i <= last; i++ {
		if t.Toks[i].Kind == token.DocComment {
			lines = append(lines, strings.TrimPrefix(strings.TrimPrefix(t.TokText(i), "///"), " "))
		}
	}
	return strings.Join(lines, "\n")
}

// FirstDocTok walks back from the last `///` token over the earlier `///` lines, which may be separated by plain comments.
func FirstDocTok(t *Tree, last uint32) uint32 {
	first := last
	for i := int(last) - 1; i > 0; i-- {
		switch t.Toks[i].Kind {
		case token.Newline, token.Comment, token.BlockComment:
			continue
		case token.DocComment:
			first = uint32(i)
			continue
		}
		break
	}
	return first
}

func depth(src []byte) int {
	n := 0
	for _, c := range src {
		switch c {
		case '{':
			n++
		case '}':
			n--
		}
	}
	return n
}
