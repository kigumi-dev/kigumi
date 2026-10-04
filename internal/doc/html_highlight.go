package doc

import (
	"html"
	"strings"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// highlightKigumi renders Kigumi source as HTML, tokenized by the compiler's
// own lexer so highlighting can't drift from the grammar.
func highlightKigumi(src string) string {
	b := []byte(src)
	var bag diag.Bag
	toks := syntax.Lex(b, &bag)
	var sb strings.Builder
	pos := 0
	for _, tk := range toks {
		if start := int(tk.Start); start > pos {
			sb.WriteString(html.EscapeString(string(b[pos:start])))
			pos = start
		}
		if tk.Kind == token.BOF || tk.Kind == token.EOF {
			continue
		}
		end := int(tk.End)
		text := string(b[pos:end])
		if class := tokenClass(tk.Kind); class != "" {
			sb.WriteString(`<span class="tok-` + class + `">` + html.EscapeString(text) + `</span>`)
		} else {
			sb.WriteString(html.EscapeString(text))
		}
		pos = end
	}
	if pos < len(b) {
		sb.WriteString(html.EscapeString(string(b[pos:])))
	}
	return sb.String()
}

func tokenClass(k token.Kind) string {
	switch {
	case k.IsKeyword():
		return "kw"
	case k.IsLiteral():
		return "lit"
	case k.IsTrivia():
		return "com"
	}
	return ""
}
