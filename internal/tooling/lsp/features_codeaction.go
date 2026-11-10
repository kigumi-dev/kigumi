package lsp

import (
	"context"
	"slices"
	"sort"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/driver"
)

// CodeAction offers the fixes the checker attached to diagnostics that
// overlap the requested range, plus the source actions an editor asks for by
// kind: source.organizeImports removes the unused imports W100 marks and
// reformats the file, the same edit `fmt --fix` would make.
func (s *Server) CodeAction(ctx context.Context, p *protocol.CodeActionParams) ([]protocol.CommandOrCodeAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []protocol.CommandOrCodeAction{}
	want := func(k protocol.CodeActionKind) bool {
		if len(p.Context.Only) == 0 {
			return true
		}
		return slices.Contains(p.Context.Only, k)
	}
	snap := s.loadDocFresh(ctx, p.TextDocument.URI)
	if snap.mod == nil {
		return out, nil
	}
	t := snap.fileOf(uriToPath(p.TextDocument.URI))
	if t == nil {
		return out, nil
	}
	if want(protocol.CodeActionKindSourceOrganizeImports) {
		if action := s.organizeImports(snap, t, p.TextDocument.URI); action != nil {
			out = append(out, action)
		}
	}
	if !want(protocol.CodeActionKindQuickFix) {
		return out, nil
	}
	from, to := offset(t.File, p.Range.Start), offset(t.File, p.Range.End)
	kind := protocol.CodeActionKindQuickFix
	for _, d := range snap.diagnosticsOf(t.File.Name) {
		if d.Loc.Span.End < from || d.Loc.Span.Start > to {
			continue
		}
		fixes := append(slices.Clone(d.Fixes), autoImportFixes(snap, t, d)...)
		for _, group := range groupFixes(fixes) {
			edit := s.fixEdit(snap, t, p.TextDocument.URI, group)
			if len(edit) == 0 {
				continue
			}
			out = append(out, &protocol.CodeAction{
				Title:       group[0].Title,
				Kind:        &kind,
				Diagnostics: []protocol.Diagnostic{s.toDiagnostic(snap, t.File, d)},
				Edit:        s.workspaceEdit(edit),
			})
		}
	}
	return out, nil
}

// organizeImports removes the imports W100 reports as unused, then formats the
// result: one whole-document edit, since the formatter decides import order.
func (s *Server) organizeImports(snap *snapshot, t *syntax.Tree, doc uri.URI) *protocol.CodeAction {
	text := string(t.File.Src)
	fixed := applyFixes(text, unusedImportFixes(t, text, snap.diagnosticsOf(t.File.Name)))
	out, err := driver.Format(driver.Source{Path: uriToPath(doc), Src: []byte(fixed)})
	if err != nil || out == text {
		return nil
	}
	kind := protocol.CodeActionKindSourceOrganizeImports
	r := protocol.Range{End: position(fileOfText(text), token.Pos(len(text)))}
	return &protocol.CodeAction{
		Title: "Organize imports",
		Kind:  &kind,
		Edit:  s.workspaceEdit(map[uri.URI][]protocol.TextEdit{doc: {{Range: r, NewText: out}}}),
	}
}

// unusedImportFixes turns the W100 warnings into edits that remove just the
// unused names, keeping the used names of a selected import list. An import
// whose bindings are all unused loses the whole declaration.
func unusedImportFixes(t *syntax.Tree, text string, ds []diag.Diagnostic) []diag.Fix {
	var unused []token.Span
	for _, d := range ds {
		if d.Code != "W100" || d.Loc.Source != 0 && d.Loc.Source != t.File.ID {
			continue
		}
		unused = append(unused, d.Loc.Span)
	}
	var out []diag.Fix
	for v := range t.List(t.Root).Views() {
		if !v.Is(syntax.ImportDecl) || !overlapsAny(v.Span(), unused) {
			continue
		}
		binding := v.AsImportDecl().Binding()
		if binding.Kind() != syntax.List {
			out = append(out, removeLinesFix(t, text, v.Span()))
			continue
		}
		list := binding.AsList()
		var kept []string
		for i := range list.Len() {
			item := t.View(list.ItemID(i))
			sp := item.Span()
			if overlapsAny(sp, unused) {
				continue
			}
			kept = append(kept, text[sp.Start:sp.End])
		}
		open, close := braceInterior(text, v.Span())
		if len(kept) == 0 || open < 0 {
			out = append(out, removeLinesFix(t, text, v.Span()))
			continue
		}
		out = append(out, diag.Fix{
			Title:   "Remove unused import",
			Loc:     diag.At(t.File, token.Span{Start: token.Pos(open), End: token.Pos(close)}),
			NewText: " " + strings.Join(kept, ", ") + " ",
		})
	}
	return out
}

// removeLinesFix removes every whole line the span touches.
func removeLinesFix(t *syntax.Tree, text string, sp token.Span) diag.Fix {
	start := int(sp.Start)
	for start > 0 && text[start-1] != '\n' {
		start--
	}
	end := int(sp.End)
	if end > len(text) {
		end = len(text)
	}
	for end < len(text) && text[end] != '\n' {
		end++
	}
	if end < len(text) {
		end++
	}
	return diag.Fix{Title: "Remove unused import", Loc: diag.At(t.File, token.Span{Start: token.Pos(start), End: token.Pos(end)})}
}

// braceInterior returns the byte offsets just inside the braces of a selected
// import list; open is one past '{' and close is the '}' itself.
func braceInterior(text string, sp token.Span) (open, close int) {
	if int(sp.Start) >= len(text) || int(sp.End) > len(text) {
		return -1, -1
	}
	inner := text[sp.Start:sp.End]
	i := strings.IndexByte(inner, '{')
	j := strings.LastIndexByte(inner, '}')
	if i < 0 || j < i {
		return -1, -1
	}
	return int(sp.Start) + i + 1, int(sp.Start) + j
}

func overlapsAny(sp token.Span, spans []token.Span) bool {
	for _, o := range spans {
		if o.Start < sp.End && sp.Start < o.End {
			return true
		}
	}
	return false
}

// applyFixes applies non-overlapping fixes right to left.
func applyFixes(text string, fixes []diag.Fix) string {
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	for _, f := range fixes {
		start, end := int(f.Loc.Span.Start), int(f.Loc.Span.End)
		if start < 0 || end < start || end > len(text) {
			continue
		}
		edits = append(edits, edit{start, end, f.NewText})
	}
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start > edits[j].start
		}
		return edits[i].end > edits[j].end
	})
	last := len(text) + 1
	for _, e := range edits {
		if e.end > last {
			continue
		}
		text = text[:e.start] + e.text + text[e.end:]
		last = e.start
	}
	return text
}

// groupFixes merges fixes of one title that edit different files, so a fix
// spanning several files is one action; two fixes of one file stay alternatives.
func groupFixes(fixes []diag.Fix) [][]diag.Fix {
	var out [][]diag.Fix
next:
	for _, f := range fixes {
		for i, g := range out {
			if g[0].Title == f.Title && !slices.ContainsFunc(g, func(o diag.Fix) bool { return o.Loc.Source == f.Loc.Source }) {
				out[i] = append(g, f)
				continue next
			}
		}
		out = append(out, []diag.Fix{f})
	}
	return out
}

// fixEdit turns a fix group into per-file edits, leaving out files the
// editor must not change (std, dependencies, generated code).
func (s *Server) fixEdit(snap *snapshot, t *syntax.Tree, doc uri.URI, group []diag.Fix) map[uri.URI][]protocol.TextEdit {
	changes := map[uri.URI][]protocol.TextEdit{}
	for _, fix := range group {
		target := s.fileOf(snap, t.File, fix.Loc)
		if o, ok := snap.originOf(target.Name); ok && !o.editable {
			continue
		}
		editURI := doc
		if target != t.File {
			editURI = uri.File(s.pathOfFile(snap, target.Name))
		}
		changes[editURI] = append(changes[editURI], protocol.TextEdit{Range: spanRange(target, fix.Loc.Span), NewText: fix.NewText})
	}
	return changes
}
