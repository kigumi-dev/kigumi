package syntax

import "strings"

// PublicDeclHead is DeclHead for a record or resource type without the fields that lack `pub` (and their doc lines), plus a note line before the closing brace.
func PublicDeclHead(t *Tree, n NodeID) string {
	if t.Kind(n) != TypeDecl {
		return DeclHead(t, n)
	}
	body := t.View(n).AsTypeDecl().Body()
	if !body.Is(RecordBody, ResourceBody) {
		return DeclHead(t, n)
	}
	first, lines := declSource(t, n)
	drop := map[int]bool{}
	for fv := range t.List(body.ID()).Views() {
		f := fv.AsField()
		if f.Vis().Valid() {
			continue
		}
		from := t.File.Line(t.Span(f.ID()).Start)
		if doc := f.AsDecl().Doc(); doc.Valid() {
			from = min(from, t.File.Line(t.Toks[FirstDocTok(t, doc.Index())].Start))
		}
		to := t.File.Line(t.Span(f.ID()).End - 1)
		for to-first+1 < len(lines) && depth([]byte(strings.Join(lines[from-first:to-first+1], "\n"))) > 0 {
			to++
		}
		for l := from; l <= to; l++ {
			drop[l] = true
		}
	}
	if len(drop) == 0 {
		return DeclHead(t, n)
	}
	var out []string
	for i, text := range lines {
		if !drop[first+i] {
			out = append(out, text)
		}
	}
	if last := len(out) - 1; last >= 0 && strings.TrimSpace(out[last]) == "}" {
		out = append(out[:last], "    // private fields omitted", out[last])
	}
	return capHead(out)
}
