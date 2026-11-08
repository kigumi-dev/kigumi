package lsp_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// A notification the server does not implement has nowhere to report its error, so the connection
// must keep serving the requests queued behind it.
func TestUnimplementedNotificationKeepsTheConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv := newServer("../../../std")
	a, b := net.Pipe()
	srv.Connect(ctx, jsonrpc2.NewStream(a))
	_, conn, server := protocol.NewClient(ctx, &liveClient{changed: make(chan struct{}, 1)}, jsonrpc2.NewStream(b))
	t.Cleanup(func() { a.Close(); b.Close(); srv.WaitIdle() })

	if err := conn.Notify(ctx, "$/unknownThing", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	doc := protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(t.TempDir(), "main.kg"))}
	if err := server.DidSave(ctx, &protocol.DidSaveTextDocumentParams{TextDocument: doc}); err != nil {
		t.Fatal(err)
	}
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown after an unimplemented notification: %v", err)
	}
	select {
	case <-conn.Done():
		t.Fatalf("the server closed the connection: %v", conn.Err())
	default:
	}
}
