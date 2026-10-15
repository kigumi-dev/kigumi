package lsp_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lsp"
)

// liveClient is the editor side of a JSON-RPC connection.
type liveClient struct {
	protocol.UnimplementedClient
	mu       sync.Mutex
	versions []int32
	changed  chan struct{}
}

func (c *liveClient) PublishDiagnostics(_ context.Context, p *protocol.PublishDiagnosticsParams) error {
	c.mu.Lock()
	v, _ := p.Version.Get()
	c.versions = append(c.versions, v)
	c.mu.Unlock()
	select {
	case c.changed <- struct{}{}:
	default:
	}
	return nil
}

func (c *liveClient) LogMessage(context.Context, *protocol.LogMessageParams) error { return nil }

func (c *liveClient) seen() []int32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int32(nil), c.versions...)
}

// Over a real JSON-RPC connection, a burst of edits publishes only the
// latest generation, in order, and a hover sent mid-burst is answered.
func TestJSONRPCEditsPublishOnlyTheLatestGeneration(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	src := "fn double(x: Int) -> Int {\n    x * 2\n}\n\nlet value = double(21)\nlet bad: String = value\n"
	path := filepath.Join(root, "main", "main.kg")
	os.WriteFile(path, []byte(src), 0o644)
	doc := uri.File(path)

	srv := lsp.New("../../std").WithDebounce(60 * time.Millisecond)
	a, b := net.Pipe()
	ctx := context.Background()
	protocol.NewServer(ctx, srv, jsonrpc2.NewStream(a))
	client := &liveClient{changed: make(chan struct{}, 1)}
	_, _, server := protocol.NewClient(ctx, client, jsonrpc2.NewStream(b))
	t.Cleanup(func() { a.Close(); b.Close(); srv.WaitIdle() })

	if _, err := server.Initialize(ctx, initParams(uri.File(root))); err != nil {
		t.Fatal(err)
	}
	server.Initialized(ctx, &protocol.InitializedParams{})
	server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Version: 1, Text: src}})
	waitFor(t, client, func(v []int32) bool { return len(v) > 0 && v[len(v)-1] == 1 })
	opened := len(client.seen())

	for v := int32(2); v <= 8; v++ {
		text := strings.Repeat("// pad\n", int(v)) + src
		server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: doc}, Version: v},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{&protocol.TextDocumentContentChangeWholeDocument{Text: text}},
		})
		if v == 5 {
			h, err := server.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: 9, Character: 13}}})
			if err != nil || h == nil {
				t.Fatalf("hover in the middle of a burst: %v %+v", err, h)
			}
		}
	}
	waitFor(t, client, func(v []int32) bool { return len(v) > opened && v[len(v)-1] == 8 })
	srv.WaitIdle()
	for i, v := range client.seen()[opened:] {
		if v != 8 {
			t.Fatalf("publish %d carries version %d, want only the latest (8): %v", i, v, client.seen())
		}
	}
}

func waitFor(t *testing.T, c *liveClient, ok func([]int32) bool) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for !ok(c.seen()) {
		select {
		case <-c.changed:
		case <-deadline:
			t.Fatalf("timed out; publishes so far: %v", c.seen())
		}
	}
}
