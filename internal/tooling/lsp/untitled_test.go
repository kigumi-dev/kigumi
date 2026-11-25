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

// Opening an unsaved buffer must not re-analyze the process's working
// directory or publish diagnostics for a file:// path that does not exist.
func TestDidOpenUntitledDoesNotAnalyzeRoot(t *testing.T) {
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "src"), 0o755))
	must(os.WriteFile(filepath.Join(root, "src", "a.kg"), []byte("fn ok() -> Void {}\n"), 0o644))
	must(os.MkdirAll(filepath.Join(root, "cmd", "app"), 0o755))
	must(os.WriteFile(filepath.Join(root, "cmd", "app", "main.kg"), []byte("print \"ok\"\n"), 0o644))
	// `.` is a module root here, so an untitled buffer that falls through to
	// the working directory would analyze it and publish file:///src/a.kg.
	t.Chdir(root)

	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := newServer(std)
	if _, err := s.Initialize(ctx, initParams(uri.File(root))); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	built := s.Counters().Built
	published := len(rec.published)

	untitled := uri.URI("untitled:Untitled-1")
	if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: untitled, Version: 1, Text: "fn main() -> Void {}\n"}}); err != nil {
		t.Fatal(err)
	}
	s.WaitIdle()

	if got := s.Counters().Built; got != built {
		t.Fatalf("opening an untitled buffer analyzed %d more roots, want 0", got-built)
	}
	if len(rec.published) != published {
		t.Fatalf("opening an untitled buffer published diagnostics: %+v", rec.published[published:])
	}
	for _, p := range rec.published {
		if strings.HasPrefix(string(p.URI), "file:///src/") {
			t.Fatalf("published diagnostics for nonexistent path %s", p.URI)
		}
	}

	// A real file still gets its diagnostics.
	path := filepath.Join(root, "src", "b.kg")
	broken := "fn broken() -> Void {\n    let x: String = 1\n}\n"
	must(os.WriteFile(path, []byte(broken), 0o644))
	doc := uri.File(path)
	if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Version: 1, Text: broken}}); err != nil {
		t.Fatal(err)
	}
	s.WaitIdle()
	found := false
	for _, p := range rec.published {
		if p.URI == doc && len(p.Diagnostics) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("normal file diagnostics missing: %+v", rec.published)
	}
}
