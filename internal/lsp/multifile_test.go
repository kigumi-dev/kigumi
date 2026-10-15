package lsp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/diag"
	"kigumi/internal/token"
)

// applyEdits applies LSP edits to text; edits must not overlap.
func applyEdits(t *testing.T, text string, edits []protocol.TextEdit) string {
	t.Helper()
	f := fileOfText(text)
	type span struct {
		start, end token.Pos
		text       string
	}
	var spans []span
	for _, e := range edits {
		spans = append(spans, span{offset(f, e.Range.Start), offset(f, e.Range.End), e.NewText})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start > spans[j].start })
	for _, sp := range spans {
		text = text[:sp.start] + sp.text + text[sp.end:]
	}
	return text
}

func TestRenameTouchesEveryFileOfTheModule(t *testing.T) {
	for _, versioned := range []bool{false, true} {
		root := t.TempDir()
		copyTree(t, "../../examples/repostat", root)
		s, ctx, _ := startServer(t, root, 0)
		s.documentChanges = versioned
		parse := filepath.Join(root, "parse", "parse.kg")
		report := filepath.Join(root, "report", "report.kg")
		parseSrc, _ := os.ReadFile(parse)
		reportSrc, _ := os.ReadFile(report)
		openDoc(s, ctx, uri.File(parse), 3, string(parseSrc))
		openDoc(s, ctx, uri.File(report), 7, string(reportSrc))

		edit, err := s.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at(uri.File(parse), pos(t, string(parseSrc), "fn tally", 0, 7)), NewName: "tallyAll"})
		if err != nil || edit == nil {
			t.Fatalf("rename: %v %+v", err, edit)
		}
		changes := map[uri.URI][]protocol.TextEdit{}
		if !versioned {
			changes = edit.Changes
		} else {
			for _, dc := range edit.DocumentChanges {
				de := dc.(*protocol.TextDocumentEdit)
				want := map[uri.URI]int32{uri.File(parse): 3, uri.File(report): 7}[de.TextDocument.URI]
				if v := de.TextDocument.Version; v == nil || *v != want {
					t.Fatalf("edit of %s must carry the open version %d: %v", de.TextDocument.URI, want, v)
				}
				for _, e := range de.Edits {
					changes[de.TextDocument.URI] = append(changes[de.TextDocument.URI], *e.(*protocol.TextEdit))
				}
			}
		}
		if len(changes) != 2 {
			t.Fatalf("versioned=%v: rename should touch parse.kg and report.kg, touched %v", versioned, changes)
		}
		newParse := applyEdits(t, string(parseSrc), changes[uri.File(parse)])
		newReport := applyEdits(t, string(reportSrc), changes[uri.File(report)])
		if strings.Contains(newParse, "tally(") || strings.Contains(newReport, "tally(") || !strings.Contains(newReport, "{Commit, tallyAll}") {
			t.Fatalf("renamed text still mentions tally:\n%s\n%s", newParse, newReport)
		}
		os.WriteFile(parse, []byte(newParse), 0o644)
		os.WriteFile(report, []byte(newReport), 0o644)
		s.mu.Lock()
		snap := s.refresh(ctx, root, "", true)
		s.mu.Unlock()
		if snap.res == nil || snap.res.HasErrors() {
			t.Fatalf("the renamed module must still check:\n%s", snap.res.Render())
		}
	}
}

// A diagnostic stored under one file but located in another is published
// under the file it points into, with the span of that file.
func TestDiagnosticsArePublishedUnderTheirOwnFile(t *testing.T) {
	root := writeModule(t, map[string]string{
		"mod.kg":          "Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n",
		"lib/lib.kg":      "pub fn one() -> Int {\n    1\n}\n",
		"cmd/app/main.kg": "import {one} from app/lib\nprint \"${one()}\"\n",
	})
	s, ctx, c := startServer(t, root, 0)
	libFile := filepath.Join(root, "lib", "lib.kg")
	mainFile := filepath.Join(root, "cmd", "app", "main.kg")
	s.mu.Lock()
	snap := s.refresh(ctx, root, "", false)
	lib := snap.fileOf(libFile)
	mainTree := snap.fileOf(mainFile)
	if lib == nil || mainTree == nil {
		s.mu.Unlock()
		t.Fatal("files missing from the snapshot")
	}
	info := snap.res.File(mainTree)
	d := diag.Errorf(diag.At(lib.File, token.Span{Start: 7, End: 10}), "synthetic")
	info.Diags = append(info.Diags, d)
	s.publishSnapshot(ctx, snap, "")
	s.mu.Unlock()

	var got []protocol.Diagnostic
	for _, p := range c.published(uri.File(libFile)) {
		got = p.Diagnostics
	}
	if len(got) != 1 || got[0].Range.Start.Line != 0 || got[0].Range.Start.Character != 7 {
		t.Fatalf("the diagnostic should be filed under lib.kg at 0:7: %+v", got)
	}
	for _, p := range c.published(uri.File(mainFile)) {
		if len(p.Diagnostics) != 0 {
			t.Fatalf("main.kg must not carry lib.kg's diagnostic: %+v", p.Diagnostics)
		}
	}
}

// Fixes of one title that edit different files form one action with one
// WorkspaceEdit; two fixes inside one file stay separate alternatives.
func TestCodeActionGroupsFixesAcrossFiles(t *testing.T) {
	fixes := []diag.Fix{
		{Title: "Rename", Loc: diag.Location{Source: 1}, NewText: "a"},
		{Title: "Rename", Loc: diag.Location{Source: 2}, NewText: "b"},
		{Title: "Rename", Loc: diag.Location{Source: 1}, NewText: "c"},
		{Title: "Other", Loc: diag.Location{Source: 1}, NewText: "d"},
	}
	groups := groupFixes(fixes)
	if len(groups) != 3 || len(groups[0]) != 2 || groups[0][1].NewText != "b" || groups[1][0].NewText != "c" || groups[2][0].Title != "Other" {
		t.Fatalf("groups: %+v", groups)
	}
}

// Renaming through an aliased import changes the imported name and leaves the alias and its uses alone.
func TestRenameKeepsImportAliases(t *testing.T) {
	lib := "pub fn one() -> Int {\n    1\n}\n"
	app := "import {one as uno} from app/lib\nprint \"${uno()}\"\n"
	root := writeModule(t, map[string]string{
		"mod.kg":          "Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n",
		"lib/lib.kg":      lib,
		"cmd/app/main.kg": app,
	})
	s, ctx, _ := startServer(t, root, 0)
	libFile, mainFile := filepath.Join(root, "lib", "lib.kg"), filepath.Join(root, "cmd", "app", "main.kg")
	edit, err := s.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at(uri.File(libFile), pos(t, lib, "one", 0, 0)), NewName: "first"})
	if err != nil || edit == nil {
		t.Fatalf("rename: %v %+v", err, edit)
	}
	if got := applyEdits(t, lib, edit.Changes[uri.File(libFile)]); got != "pub fn first() -> Int {\n    1\n}\n" {
		t.Fatalf("lib.kg: %q", got)
	}
	if got := applyEdits(t, app, edit.Changes[uri.File(mainFile)]); got != "import {first as uno} from app/lib\nprint \"${uno()}\"\n" {
		t.Fatalf("main.kg: %q", got)
	}
}
