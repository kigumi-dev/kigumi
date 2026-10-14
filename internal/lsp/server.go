// Package lsp implements the Kigumi language server on go.lsp.dev/protocol:
// diagnostics, hover, go to definition, formatting, document symbols and
// completion, computed from the driver and the checker.
package lsp

import (
	"context"
	"io"
	"os"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/driver"
)

// Server holds the open documents and the module roots; every LSP method
// it does not override answers "method not found". go.lsp.dev runs
// handlers concurrently, so each one takes mu.
type Server struct {
	protocol.UnimplementedServer
	mu sync.Mutex
	// roots lists every workspace folder, in the order the client gave
	// them; roots[0] is where std auto-discovery looks.
	roots       []string
	stdRoot     string
	target      driver.Target
	docs        map[uri.URI]string
	docVersions map[uri.URI]int32
	published   map[string]map[uri.URI]bool
	// snaps caches one analysis per module root and entry until a
	// document or the disk changes, so hover, completion and the rest do
	// not each re-check the module.
	snaps map[string]*snapshot
	// depIndex maps a package directory to every root whose last snapshot
	// read it, for invalidateFile's reverse-dependency lookup.
	depIndex map[string]map[string]bool
	// documentChanges: client declared workspace.workspaceEdit.documentChanges.
	documentChanges bool
}

func New(stdRoot string) *Server {
	return &Server{stdRoot: stdRoot, docs: map[uri.URI]string{}, docVersions: map[uri.URI]int32{}, published: map[string]map[uri.URI]bool{}, snaps: map[string]*snapshot{}, depIndex: map[string]map[string]bool{}}
}

type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error                { return os.Stdout.Close() }

// WithTarget makes the editor enforce the allocation profiles of t like `kigumi check`.
func (s *Server) WithTarget(t driver.Target) *Server {
	s.target = t
	return s
}

// Serve speaks the protocol on stdin/stdout until the client hangs up.
func Serve(stdRoot string, target driver.Target) error {
	_, conn, _ := protocol.NewServer(context.Background(), New(stdRoot).WithTarget(target), jsonrpc2.NewStream(stdio{}))
	<-conn.Done()
	if err := conn.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (s *Server) Shutdown(context.Context) error                           { return nil }
func (s *Server) Exit(context.Context) error                               { return nil }
func (s *Server) SetTrace(context.Context, *protocol.SetTraceParams) error { return nil }
