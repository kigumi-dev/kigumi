package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestEditorReportsACopyOnlyThePlanFinds(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "noalloc fn sameStart(a: Bytes, b: Bytes) -> Bool {\n    a.startsWith(b)\n}\n\nfn main() -> Unit! {\n    print \"${sameStart(\"abc\".toBytes(), \"ab\".toBytes())}\"\n}\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := newServer(std)
	doc := uri.File(path)
	if _, err := s.Initialize(ctx, initParams(uri.File(root))); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Version: 1, Text: src}})
	var last *protocol.PublishDiagnosticsParams
	for _, p := range rec.published {
		if p.URI == doc {
			last = p
		}
	}
	if last == nil || len(last.Diagnostics) != 1 || last.Diagnostics[0].Code != protocol.String("E1700") || last.Diagnostics[0].Range.Start.Line != 1 {
		t.Fatalf("want one E1700 at `a.startsWith(b)`, got %+v", last)
	}
}
