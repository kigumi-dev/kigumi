package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lsp"
)

// TestCapabilitiesMatchImplementation: a capability is advertised only
// when the server actually behaves that way.
func TestCapabilitiesMatchImplementation(t *testing.T) {
	root := t.TempDir()
	std, _ := filepath.Abs("../../std")
	s := lsp.New(std)
	res, err := s.Initialize(protocol.WithClient(context.Background(), &recorder{}), initParams(uri.File(root)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Capabilities.DocumentRangeFormattingProvider != nil {
		t.Fatalf("range formatting must not be advertised: Formatting always reformats the whole file")
	}
	if res.Capabilities.DocumentFormattingProvider == nil {
		t.Fatalf("whole-file formatting should still be advertised")
	}
	if res.Capabilities.Workspace == nil || res.Capabilities.Workspace.WorkspaceFolders == nil || res.Capabilities.Workspace.WorkspaceFolders.Supported == nil || !*res.Capabilities.Workspace.WorkspaceFolders.Supported {
		t.Fatalf("workspace folder support should be advertised: %+v", res.Capabilities.Workspace)
	}
}

// TestInitializeRootURIFallback: a client with no workspaceFolders support
// still gets its files analyzed, through the deprecated rootUri field.
func TestInitializeRootURIFallback(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "let bad: String = 1\n"
	os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := lsp.New(std)
	p := &protocol.InitializeParams{}
	rootURI := uri.File(root)
	p.RootURI = &rootURI
	if _, err := s.Initialize(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	if len(rec.published) != 1 || len(rec.published[0].Diagnostics) == 0 {
		t.Fatalf("rootUri fallback should still analyze the workspace: %+v", rec.published)
	}
}

// TestInitializeRootPathFallback: rootPath is used when a client gives
// neither workspaceFolders nor rootUri.
func TestInitializeRootPathFallback(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte("let bad: String = 1\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := lsp.New(std)
	p := &protocol.InitializeParams{}
	p.RootPath = protocol.NewNullable(root)
	if _, err := s.Initialize(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	if len(rec.published) != 1 || len(rec.published[0].Diagnostics) == 0 {
		t.Fatalf("rootPath fallback should still analyze the workspace: %+v", rec.published)
	}
}

// TestMultiRootWorkspace: every workspace folder is analyzed, not just the first.
func TestMultiRootWorkspace(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(a, "main"), 0o755)
	os.MkdirAll(filepath.Join(b, "main"), 0o755)
	os.WriteFile(filepath.Join(a, "main", "main.kg"), []byte("let bad: String = 1\n"), 0o644)
	os.WriteFile(filepath.Join(b, "main", "main.kg"), []byte("let also_bad: Int = \"x\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := lsp.New(std)
	p := &protocol.InitializeParams{}
	p.WorkspaceFolders = protocol.NewNullable([]protocol.WorkspaceFolder{{URI: uri.File(a), Name: "a"}, {URI: uri.File(b), Name: "b"}})
	s.Initialize(ctx, p)
	s.Initialized(ctx, &protocol.InitializedParams{})
	seen := map[uri.URI]bool{}
	for _, pub := range rec.published {
		if len(pub.Diagnostics) > 0 {
			seen[pub.URI] = true
		}
	}
	wantA, wantB := uri.File(filepath.Join(a, "main", "main.kg")), uri.File(filepath.Join(b, "main", "main.kg"))
	if !seen[wantA] || !seen[wantB] {
		t.Fatalf("both workspace folders should be analyzed: %+v", rec.published)
	}
}

// TestRenameVersionedDocumentChanges: a client that declares documentChanges
// support gets versioned edits back, not a bare, unversioned edit map.
func TestRenameVersionedDocumentChanges(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "fn double(x: Int) -> Int {\n    x * 2\n}\n\nlet value = double(21)\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	ctx := protocol.WithClient(context.Background(), &recorder{})
	s := lsp.New(std)
	p := initParams(uri.File(root))
	docChanges := true
	p.Capabilities.Workspace = &protocol.WorkspaceClientCapabilities{WorkspaceEdit: &protocol.WorkspaceEditClientCapabilities{DocumentChanges: &docChanges}}
	s.Initialize(ctx, p)
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Text: src, Version: 3}})
	at := protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: 0, Character: 4}}
	edit, err := s.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at, NewName: "twice"})
	if err != nil || edit == nil {
		t.Fatalf("rename: %v %+v", err, edit)
	}
	if edit.Changes != nil {
		t.Fatalf("a documentChanges-capable client must get DocumentChanges, not plain Changes: %+v", edit.Changes)
	}
	if len(edit.DocumentChanges) != 1 {
		t.Fatalf("expected one changed document: %+v", edit.DocumentChanges)
	}
	tde, ok := edit.DocumentChanges[0].(*protocol.TextDocumentEdit)
	if !ok {
		t.Fatalf("expected a TextDocumentEdit: %+v", edit.DocumentChanges[0])
	}
	if tde.TextDocument.URI != doc || tde.TextDocument.Version == nil || *tde.TextDocument.Version != 3 {
		t.Fatalf("edit must carry the open document's version: %+v", tde.TextDocument)
	}
	if len(tde.Edits) != 2 {
		t.Fatalf("expected the declaration and its one use: %+v", tde.Edits)
	}
}

// TestRenameCollision: renaming onto an existing name in the same package
// must fail instead of producing a broken edit.
func TestRenameCollision(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "fn double(x: Int) -> Int {\n    x * 2\n}\n\nfn triple(x: Int) -> Int {\n    x * 3\n}\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	ctx := protocol.WithClient(context.Background(), &recorder{})
	s := lsp.New(std)
	s.Initialize(ctx, initParams(uri.File(root)))
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Text: src}})
	at := protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: 0, Character: 4}}
	if _, err := s.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at, NewName: "triple"}); err == nil {
		t.Fatalf("renaming double to the already-declared triple must fail")
	}
}

// TestFormattingError: a format failure comes back as an error, not a
// silent empty-edit "no change needed".
func TestFormattingError(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	broken := "fn broken(x: Int -> Int {\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(broken), 0o644)
	std, _ := filepath.Abs("../../std")
	ctx := protocol.WithClient(context.Background(), &recorder{})
	s := lsp.New(std)
	s.Initialize(ctx, initParams(uri.File(root)))
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Text: broken}})
	edits, err := s.Formatting(ctx, &protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}})
	if err == nil {
		t.Fatalf("formatting an unparseable file must return an error, not %+v", edits)
	}
	if !strings.Contains(err.Error(), "expected") && !strings.Contains(err.Error(), "error") {
		t.Fatalf("format error should carry the parser's message: %v", err)
	}
}
