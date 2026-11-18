package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func renameAt(t *testing.T, source string, line, character uint32, newName string) (string, error) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "main.kg")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	ctx := protocol.WithClient(context.Background(), &recorder{})
	server := newServer(std)
	if _, err := server.Initialize(ctx, initParams(uri.File(root))); err != nil {
		t.Fatal(err)
	}
	doc := uri.File(path)
	if err := server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Text: source}}); err != nil {
		t.Fatal(err)
	}
	position := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: doc},
		Position:     protocol.Position{Line: line, Character: character},
	}
	edit, err := server.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: position, NewName: newName})
	if err != nil {
		return "", err
	}
	if edit == nil {
		t.Fatalf("rename answered nothing at %d:%d", line, character)
	}
	edits := edit.Changes[doc]
	sort.Slice(edits, func(i, j int) bool {
		a, b := edits[i].Range.Start, edits[j].Range.Start
		return a.Line > b.Line || a.Line == b.Line && a.Character > b.Character
	})
	lines := strings.Split(source, "\n")
	for _, e := range edits {
		line := lines[e.Range.Start.Line]
		lines[e.Range.Start.Line] = line[:e.Range.Start.Character] + e.NewText + line[e.Range.End.Character:]
	}
	return strings.Join(lines, "\n"), nil
}
