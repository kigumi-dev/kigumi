package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestRenamePreservesBindings(t *testing.T) {
	cases := []struct {
		name, source, newName string
		allowed               bool
	}{
		{"same-block", "fn calc() -> Int { let §first = 1; let second = 2; first + second }", "second", false},
		{"parameter", "fn calc(§first: Int, second: Int) -> Int { first + second }", "second", false},
		{"inner-capture", "fn calc() -> Int { let §first = 1; { let second = 2; first + second } }", "second", false},
		{"reverse-capture", "const second: Int = 2\nfn calc() -> Int { let §first = 1; first + second }", "second", false},
		{"lambda-capture", "fn calc() -> Int { let §first = 1; let f = (second: Int) => first + second; f(2) }", "second", false},
		{"initializer", "fn calc() -> Int { let §first = 1; let second = first + 1; first + second }", "second", false},
		{"function-capture", "fn §first() -> Int { 1 }\nfn calc() -> Int { let second = () => 2; first() + second() }", "second", false},
		{"constant-capture", "const §first: Int = 1\nfn calc() -> Int { let second = 2; first + second }", "second", false},
		{"type-capture", "type §First = { value Int }\nfn calc[Second]() -> First? { None }", "Second", false},
		{"later-shadow", "fn calc() -> Int { let §longName = 1; let result = longName; let second = 2; result + second }", "second", false},
		{"disjoint-scope", "fn calc() -> Int { let §first = 1; let result = { let second = 2; second }; first + result }", "second", true},
		{"outer-name", "const second: Int = 2\nfn calc() -> Int { let §first = 1; first }\nprint \"${second}\"", "second", true},
		{"unicode", "fn calc() -> Int { let 日本語 = 2; let §first = 1; 日本語 + first }", "二", true},
		{"unchanged", "fn calc() -> Int { let §first = 1; first }", "first", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := strings.Index(tc.source, "§")
			prefix := tc.source[:at]
			source := strings.Replace(tc.source, "§", "", 1) + "\n"
			line := strings.Count(prefix, "\n")
			column := prefix[strings.LastIndex(prefix, "\n")+1:]
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
				Position:     protocol.Position{Line: uint32(line), Character: uint32(len(utf16.Encode([]rune(column))))},
			}
			edit, err := server.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: position, NewName: tc.newName})
			if !tc.allowed {
				if err == nil || edit != nil {
					t.Fatalf("unsafe rename returned edit=%+v err=%v", edit, err)
				}
				return
			}
			if err != nil || edit == nil || len(edit.Changes[doc]) < 2 {
				t.Fatalf("safe rename returned edit=%+v err=%v", edit, err)
			}
			for _, change := range edit.Changes[doc] {
				if change.NewText != tc.newName {
					t.Errorf("replacement=%q, want %q", change.NewText, tc.newName)
				}
			}
		})
	}
}
