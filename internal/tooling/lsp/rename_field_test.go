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

func TestRenameFieldAcrossLiteralAndPattern(t *testing.T) {
	source := "type P = { first Int, other Int }\n" +
		"fn f(p: P) -> Int { p.first }\n" +
		"fn g(p: P) -> Int { match p { P { first: a, other: b } -> a + b } }\n" +
		"fn h(x: Int) -> P { P { first: x, other: 2 } }\n" +
		"print \"${f(P { first: 1, other: 2 })} ${g(P { first: 3, other: 4 })} ${f(h(5))}\"\n"
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
		Position:     protocol.Position{Line: 0, Character: 11},
	}
	edit, err := server.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: position, NewName: "second"})
	if err != nil || edit == nil {
		t.Fatalf("field rename: edit=%+v err=%v", edit, err)
	}
	edits := edit.Changes[doc]
	sort.Slice(edits, func(i, j int) bool {
		a, b := edits[i].Range.Start, edits[j].Range.Start
		return a.Line > b.Line || a.Line == b.Line && a.Character > b.Character
	})
	lines := strings.Split(source, "\n")
	for _, e := range edits {
		if e.Range.Start.Line != e.Range.End.Line || e.Range.End.Character-e.Range.Start.Character != 5 {
			t.Fatalf("edit does not cover only the field name: %+v", e)
		}
		line := lines[e.Range.Start.Line]
		lines[e.Range.Start.Line] = line[:e.Range.Start.Character] + e.NewText + line[e.Range.End.Character:]
	}
	got := strings.Join(lines, "\n")
	if got != strings.ReplaceAll(source, "first", "second") {
		t.Fatalf("renamed source differs beyond the field name:\n%s", got)
	}
}
