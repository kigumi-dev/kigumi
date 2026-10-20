package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func decodeSemanticTokens(data []uint32) [][5]uint32 {
	var out [][5]uint32
	line, char := uint32(0), uint32(0)
	for i := 0; i+4 < len(data); i += 5 {
		dl, dc, length, typ, mods := data[i], data[i+1], data[i+2], data[i+3], data[i+4]
		if dl > 0 {
			line += dl
			char = dc
		} else {
			char += dc
		}
		out = append(out, [5]uint32{line, char, length, typ, mods})
	}
	return out
}

// TestSemanticTokens: a declaration and a shadowing local are highlighted
// from the checker's own entity table, which a lexical grammar cannot do.
func TestSemanticTokens(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "fn double(x: Int) -> Int {\n    x * 2\n}\n\nlet value = double(21)\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	ctx := protocol.WithClient(context.Background(), &recorder{})
	s := newServer(std)
	s.Initialize(ctx, initParams(uri.File(root)))
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Text: src}})

	toks, err := s.SemanticTokensFull(ctx, &protocol.SemanticTokensParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}})
	if err != nil || toks == nil {
		t.Fatalf("semantic tokens: %v %+v", err, toks)
	}
	rows := decodeSemanticTokens(toks.Data)
	const (
		typFunction = 8
		modDecl     = 1
	)
	foundDecl, foundParam := false, false
	for _, r := range rows {
		if r[0] == 0 && r[3] == typFunction && r[4]&modDecl != 0 {
			foundDecl = true
		}
		if r[0] == 0 && r[3] == 4 { // typeParameter/parameter row for x
			foundParam = true
		}
	}
	if !foundDecl {
		t.Fatalf("expected a declared-function token on line 0: %v", rows)
	}
	if !foundParam {
		t.Fatalf("expected a parameter token on line 0: %v", rows)
	}
}
