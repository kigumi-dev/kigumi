package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/tooling/lsp"
)

const profileCopySrc = "type Nm = {\n    pub name String\n    pub mut n Int\n}\n\nfn dup(o: Nm?) -> Int {\n    let mut b = o\n    match &mut b {\n        Some(x) -> { x.n = 2 }\n        None -> {}\n    }\n    if o is Some(_) { 1 } else { 0 }\n}\n\nfn main() -> Void! {\n    let o: Nm? = None\n    let n = dup(o)\n}\n"

func withProfile(allocator, profile string) driver.Target {
	t := driver.HostTarget()
	t.Allocator, t.Profile = allocator, profile
	return t
}

func profileDiagnostics(t *testing.T, s *lsp.Server, opts string) []protocol.Diagnostic {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(profileCopySrc), 0o644)
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	params := initParams(uri.File(root))
	if opts != "" {
		params.InitializationOptions = []byte(opts)
	}
	if _, err := s.Initialize(ctx, params); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Version: 1, Text: profileCopySrc}})
	var last *protocol.PublishDiagnosticsParams
	for _, p := range rec.published {
		if p.URI == doc {
			last = p
		}
	}
	if last == nil {
		t.Fatal("no diagnostics published")
	}
	return last.Diagnostics
}

func TestEditorReportsAProfileCopy(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	for _, tc := range []struct {
		name   string
		server *lsp.Server
		opts   string
		code   string
	}{
		{"allocator flag", newServer(std).WithTarget(withProfile("none", "")), "", "E959"},
		{"allocator option", newServer(std), `{"allocator":"none"}`, "E959"},
		{"profile flag", newServer(std).WithTarget(withProfile("", "recoverable-only")), "", "E1280"},
		{"profile option", newServer(std), `{"profile":"recoverable-only"}`, "E1280"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := profileDiagnostics(t, tc.server, tc.opts)
			if len(diags) != 1 || diags[0].Code != protocol.String(tc.code) || diags[0].Range.Start.Line != 16 {
				t.Fatalf("want one %s at `dup(o)`, got %+v", tc.code, diags)
			}
		})
	}
	t.Run("default", func(t *testing.T) {
		if diags := profileDiagnostics(t, newServer(std), ""); len(diags) != 0 {
			t.Fatalf("the default profile allows the copy, got %+v", diags)
		}
	})
}
