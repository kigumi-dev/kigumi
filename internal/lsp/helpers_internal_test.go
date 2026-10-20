package lsp

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// memClient records publishes from any goroutine.
type memClient struct {
	protocol.UnimplementedClient
	mu   sync.Mutex
	pubs []*protocol.PublishDiagnosticsParams
}

func (c *memClient) PublishDiagnostics(_ context.Context, p *protocol.PublishDiagnosticsParams) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pubs = append(c.pubs, p)
	return nil
}

func (c *memClient) LogMessage(context.Context, *protocol.LogMessageParams) error { return nil }

func (c *memClient) published(u uri.URI) []*protocol.PublishDiagnosticsParams {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []*protocol.PublishDiagnosticsParams
	for _, p := range c.pubs {
		if p.URI == u {
			out = append(out, p)
		}
	}
	return out
}

func (c *memClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pubs)
}

// writeModule writes files (relative path to content) under a temp root and returns it.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func startServer(t *testing.T, root string, debounce time.Duration) (*Server, context.Context, *memClient) {
	t.Helper()
	std, _ := filepath.Abs("../../std")
	c := &memClient{}
	ctx := protocol.WithClient(context.Background(), c)
	s := newServer(std).WithDebounce(debounce)
	p := &protocol.InitializeParams{}
	p.WorkspaceFolders = protocol.NewNullable([]protocol.WorkspaceFolder{{URI: uri.File(root), Name: "root"}})
	if _, err := s.Initialize(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.Initialized(ctx, &protocol.InitializedParams{})
	t.Cleanup(func() { s.Shutdown(ctx); s.WaitIdle() })
	return s, ctx, c
}

func openDoc(s *Server, ctx context.Context, u uri.URI, version int32, text string) {
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: u, Version: version, Text: text}})
}

func editDoc(s *Server, ctx context.Context, u uri.URI, version int32, text string) {
	s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: u}, Version: version},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{&protocol.TextDocumentContentChangeWholeDocument{Text: text}},
	})
}

func wait(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}
