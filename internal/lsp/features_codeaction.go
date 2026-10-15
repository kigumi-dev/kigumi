package lsp

import (
	"context"
	"slices"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

// CodeAction offers the fixes the checker attached to diagnostics that
// overlap the requested range.
func (s *Server) CodeAction(ctx context.Context, p *protocol.CodeActionParams) ([]protocol.CommandOrCodeAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []protocol.CommandOrCodeAction{}
	snap := s.loadDocFresh(ctx, p.TextDocument.URI)
	if snap.mod == nil {
		return out, nil
	}
	t := snap.fileOf(uriToPath(p.TextDocument.URI))
	if t == nil {
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
