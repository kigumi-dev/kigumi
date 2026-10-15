package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lsp"
)

func TestEditorReportsABoxOnlyTheBuildFinds(t *testing.T) {
	for _, lazy := range []bool{false, true} {
		t.Run(map[bool]string{false: "eager", true: "lazy-std"}[lazy], func(t *testing.T) { editorReportsABox(t, lazy) })
	}
}

func editorReportsABox(t *testing.T, lazyStd bool) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "type Point = {\n    pub x Int\n    pub y Int\n}\n\nfn size[T](v: &T) -> Int { 1 }\n\nnoalloc fn lend[T](v: T) -> Int { size(&v) }\n\nnoalloc fn span(p: Point) -> Int {\n    lend(p)\n}\n\nprint \"${span(Point { x: 1, y: 2 })}\"\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := lsp.New(std).WithLazyStd(lazyStd)
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
	if last == nil || len(last.Diagnostics) != 1 || last.Diagnostics[0].Code != protocol.String("E1570") || last.Diagnostics[0].Range.Start.Line != 10 {
		t.Fatalf("want one E1570 at `lend(p)`, got %+v", last)
	}
	syms, _ := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}})
	seen := map[string]bool{}
	for _, sym := range syms.(protocol.DocumentSymbolSlice) {
		if seen[sym.Name] {
			t.Errorf("%s is listed twice", sym.Name)
		}
		seen[sym.Name] = true
	}
}
