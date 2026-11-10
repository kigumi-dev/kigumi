package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// A module's build.kg is checked and published, including edits still only in
// the editor overlay, without running configure or the build graph.
func TestBuildFileDiagnostics(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	main := "let x = 1\nprint \"${x}\"\n"
	mainPath := filepath.Join(root, "main.kg")
	os.WriteFile(mainPath, []byte(main), 0o644)
	buildPath := filepath.Join(root, "build.kg")
	valid := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let app = b.executable(\"app\", \"\", b.request().target())?\n}\n"
	os.WriteFile(buildPath, []byte(valid), 0o644)

	std, _ := filepath.Abs("../../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := newServer(std)
	if _, err := s.Initialize(ctx, initParams(uri.File(root))); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: uri.File(mainPath), Version: 1, Text: main}})
	s.WaitIdle()

	buildDoc := uri.File(buildPath)
	broken := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let n: Int = \"x\"\n}\n"
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: buildDoc, Version: 1, Text: broken}})
	s.WaitIdle()
	last := lastDiagnostics(rec, buildDoc)
	if last == nil || len(last.Diagnostics) == 0 {
		t.Fatalf("no build.kg diagnostics published: %+v", last)
	}
	if !strings.Contains(string(last.Diagnostics[0].Message.(protocol.String)), "Int") || last.Diagnostics[0].Range.Start.Line != 3 {
		t.Fatalf("diagnostic should point into build.kg: %+v", last.Diagnostics[0])
	}

	s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: buildDoc}, Version: 2},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{&protocol.TextDocumentContentChangeWholeDocument{Text: valid}},
	})
	s.WaitIdle()
	if last := lastDiagnostics(rec, buildDoc); last == nil || len(last.Diagnostics) != 0 {
		t.Fatalf("fixed build.kg should clear: %+v", last)
	}
}

// source.organizeImports removes unused imports and formats, matching fmt --fix.
func TestOrganizeImportsCodeAction(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	src := "import json from std/json\nimport text from std/text\n\nlet  s = text.trim(\" a \")\nprint \"${s}\"\n"
	path := filepath.Join(root, "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := newServer(std)
	if _, err := s.Initialize(ctx, initParams(uri.File(root))); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Version: 1, Text: src}})
	s.WaitIdle()

	kind := protocol.CodeActionKindSourceOrganizeImports
	actions, err := s.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: doc},
		Context:      protocol.CodeActionContext{Only: []protocol.CodeActionKind{kind}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got *protocol.CodeAction
	for _, a := range actions {
		if ca := a.(*protocol.CodeAction); ca.Kind != nil && *ca.Kind == kind {
			got = ca
		}
	}
	if got == nil {
		t.Fatalf("no organizeImports action: %+v", actions)
	}
	edits := got.Edit.Changes[doc]
	if len(edits) != 1 {
		t.Fatalf("want one whole-document edit, got %+v", edits)
	}
	if strings.Contains(edits[0].NewText, "std/json") {
		t.Errorf("unused import not removed:\n%s", edits[0].NewText)
	}
	if !strings.Contains(edits[0].NewText, "import text from std/text") || !strings.Contains(edits[0].NewText, "let s = text.trim") {
		t.Errorf("result should keep the used import and format:\n%s", edits[0].NewText)
	}
}

// Organize imports keeps the used names of a selected import list and drops
// the declarations whose bindings are all unused.
func TestOrganizeImportsKeepsUsedSelectedNames(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	src := "import {floor, sqrt} from std/math\nimport {decode, encode} from std/json\n\nlet x = sqrt(4.0)\nprint \"${x}\"\n"
	path := filepath.Join(root, "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := newServer(std)
	if _, err := s.Initialize(ctx, initParams(uri.File(root))); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Version: 1, Text: src}})
	s.WaitIdle()

	kind := protocol.CodeActionKindSourceOrganizeImports
	actions, err := s.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: doc},
		Context:      protocol.CodeActionContext{Only: []protocol.CodeActionKind{kind}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got *protocol.CodeAction
	for _, a := range actions {
		if ca := a.(*protocol.CodeAction); ca.Kind != nil && *ca.Kind == kind {
			got = ca
		}
	}
	if got == nil {
		t.Fatalf("no organizeImports action: %+v", actions)
	}
	edits := got.Edit.Changes[doc]
	if len(edits) != 1 {
		t.Fatalf("want one whole-document edit, got %+v", edits)
	}
	if strings.Contains(edits[0].NewText, "floor") || strings.Contains(edits[0].NewText, "std/json") {
		t.Errorf("unused imports not removed:\n%s", edits[0].NewText)
	}
	if !strings.Contains(edits[0].NewText, "import {sqrt} from std/math") {
		t.Errorf("the used selected name must stay:\n%s", edits[0].NewText)
	}
}

func lastDiagnostics(rec *recorder, doc uri.URI) *protocol.PublishDiagnosticsParams {
	var last *protocol.PublishDiagnosticsParams
	for _, p := range rec.published {
		if p.URI == doc {
			last = p
		}
	}
	return last
}
