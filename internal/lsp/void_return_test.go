package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestVoidReturnForms(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "fn cleanup() {}\n\nfn run() ! {}\n\nfn main() {\n    cleanup()\n    let r = run()\n}\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	s := newServer(std)
	s.Initialize(ctx, initParams(uri.File(root)))
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Version: 1, Text: src}})
	hoverAt := func(line, col uint32) string {
		h, _ := s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: line, Character: col}}})
		if h == nil {
			return ""
		}
		return h.Contents.(*protocol.MarkupContent).Value
	}
	if got := hoverAt(5, 6); got != "```kigumi\nfn cleanup()\n```" {
		t.Errorf("omitted return hover: %q", got)
	}
	if got := hoverAt(6, 8); got != "```kigumi\nlocal r: Void!\n```" {
		t.Errorf("bare ! result type: %q", got)
	}
	edits, err := s.Formatting(ctx, &protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}})
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 0 {
		t.Errorf("canonical source should not be reformatted: %+v", edits)
	}
	for _, p := range rec.published {
		for _, d := range p.Diagnostics {
			if d.Severity == protocol.DiagnosticSeverityError {
				t.Errorf("unexpected error: %s", d.Message)
			}
		}
	}
}
