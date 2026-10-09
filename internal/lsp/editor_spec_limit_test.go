package lsp_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lsp"
)

func lastPublished(t *testing.T, src string) *protocol.PublishDiagnosticsParams {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := lsp.New(std)
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
	return last
}

func TestEditorReportsASpecializationRunaway(t *testing.T) {
	src := "type Box[T] = {\n    pub v T\n}\n\nfn Box[T: Copy].nest(self, n: Int) -> Int {\n    if n == 0 { 0 } else { 1 + Box { v: Box { v: self.v } }.nest(n - 1) }\n}\n\nprint \"${Box { v: (1, 2) }.nest(3)}\"\n"
	last := lastPublished(t, src)
	if last == nil || len(last.Diagnostics) != 1 {
		t.Fatalf("want one W1710, got %+v", last)
	}
	d := last.Diagnostics[0]
	if d.Code != protocol.String("W1710") || d.Severity != protocol.DiagnosticSeverityWarning || d.Range.Start.Line != 5 {
		t.Fatalf("want W1710 as a warning at the recursive call, got %+v", d)
	}
}

func TestEditorReportsASpecializationBudget(t *testing.T) {
	var src strings.Builder
	src.WriteString("type P = {\n    pub x Int\n    pub y Int\n}\n\n")
	for i := 0; i < 11; i++ {
		fmt.Fprintf(&src, "fn g%d[T: Copy](x: T) -> Int { g%d((x, 1)) + g%d((2, x)) }\n", i, i+1, i+1)
	}
	src.WriteString("fn g11[T: Copy](x: T) -> Int { 1 }\n\nprint \"${g0(P { x: 1, y: 2 })}\"\n")
	last := lastPublished(t, src.String())
	if last == nil || len(last.Diagnostics) != 1 {
		t.Fatalf("want one W1711, got %+v", last)
	}
	if d := last.Diagnostics[0]; d.Code != protocol.String("W1711") || d.Severity != protocol.DiagnosticSeverityWarning {
		t.Fatalf("want W1711 as a warning, got %+v", d)
	}
}

func TestEditorSaysNothingAboutScalarGenerics(t *testing.T) {
	src := "fn id[T](x: T) -> T { x }\n\nprint \"${id(3)}\"\n"
	if last := lastPublished(t, src); last != nil && len(last.Diagnostics) != 0 {
		t.Fatalf("want no diagnostics, got %+v", last)
	}
}
