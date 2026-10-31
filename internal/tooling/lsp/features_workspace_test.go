package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// TestSymbolsDedupsOverlappingWorkspaceFolders: adding both a folder and a
// module root nested inside it must not return each symbol twice.
func TestSymbolsDedupsOverlappingWorkspaceFolders(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "nested")
	os.MkdirAll(filepath.Join(inner, "main"), 0o755)
	os.WriteFile(filepath.Join(inner, "mod.kg"), []byte("Module {\n    name: \"nested\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(inner, "main", "main.kg"), []byte("fn f() -> Int {\n    1\n}\n"), 0o644)

	std, _ := filepath.Abs("../../../std")
	ctx := protocol.WithClient(context.Background(), &recorder{})
	s := newServer(std)
	p := &protocol.InitializeParams{}
	p.WorkspaceFolders = protocol.NewNullable([]protocol.WorkspaceFolder{{URI: uri.File(outer), Name: "outer"}, {URI: uri.File(inner), Name: "inner"}})
	s.Initialize(ctx, p)

	syms, _ := s.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: "f"})
	list := syms.(protocol.SymbolInformationSlice)
	if len(list) != 1 {
		t.Fatalf("expected one symbol for f, got %d: %+v", len(list), list)
	}
}
