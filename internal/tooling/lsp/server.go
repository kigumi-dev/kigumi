// Package lsp implements the Kigumi language server on go.lsp.dev/protocol:
// diagnostics, hover, go to definition, formatting, document symbols and
// completion, computed from the driver and the checker.
package lsp

import (
	"context"
	stderrors "errors"
	"io"
	"os"
	"sync"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	buildconfig "kigumi/internal/toolchain/config"
)

// Server protects documents and snapshots from background analysis with mu; Connect orders incoming messages.
type Server struct {
	protocol.UnimplementedServer
	mu sync.Mutex
	// roots lists every workspace folder, in the order the client gave
	// them; roots[0] is where std auto-discovery looks.
	roots       []string
	stdRoot     string
	target      buildconfig.Target
	docs        map[uri.URI]string
	docVersions map[uri.URI]int32
	published   map[string]map[uri.URI]bool
	// snaps caches one analysis per module root and entry until a
	// document or the disk changes, so hover, completion and the rest do
	// not each re-check the module.
	snaps map[string]*snapshot
	// last is each root's most recent complete snapshot; requests answer
	// from it while a newer analysis is pending or running.
	last   map[string]*snapshot
	states map[string]*rootState
	// pending lists the roots to analyze when the debounce timer fires,
	// with the document to publish for even when it has no diagnostics.
	pending  map[string]uri.URI
	timer    *time.Timer
	bg       context.Context
	debounce time.Duration
	work     sync.WaitGroup
	counts   counters
	cache    string
	compiler buildconfig.CompilerOptions
	lazyStd  bool
	docSyms  map[uri.URI]docSymbols
	pkgSyms  map[string]pkgSymbols
	hook     func(stage string, in *analysisInput)
	// depIndex maps a package directory to every root whose last snapshot
	// read it, for invalidateFile's reverse-dependency lookup.
	depIndex map[string]map[string]bool
	// documentChanges: client declared workspace.workspaceEdit.documentChanges.
	documentChanges bool
	life            *lifecycle
	shiftMu         sync.Mutex
	shifts          map[uri.URI]*shiftEntry
}

const DefaultDebounce = 150 * time.Millisecond

func New(stdRoot string) *Server {
	return &Server{stdRoot: stdRoot, docs: map[uri.URI]string{}, docVersions: map[uri.URI]int32{}, published: map[string]map[uri.URI]bool{}, snaps: map[string]*snapshot{}, last: map[string]*snapshot{}, states: map[string]*rootState{}, pending: map[string]uri.URI{}, debounce: DefaultDebounce, depIndex: map[string]map[string]bool{}, life: newLifecycle(), docSyms: map[uri.URI]docSymbols{}, pkgSyms: map[string]pkgSymbols{}}
}

// WithDebounce sets the edit coalescing window; zero analyzes inside DidChange.
func (s *Server) WithDebounce(d time.Duration) *Server {
	s.debounce = d
	return s
}

func (s *Server) WithPackageCache(dir string, o buildconfig.CompilerOptions) *Server {
	s.cache, s.compiler = dir, o
	return s
}

// WithLazyStd lets the MIR diagnostics run skip the std bodies a module never names.
func (s *Server) WithLazyStd(on bool) *Server {
	s.lazyStd = on
	return s
}

type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error                { return os.Stdout.Close() }

// WithTarget makes the editor enforce the allocation profiles of t like `kigumi check`.
func (s *Server) WithTarget(t buildconfig.Target) *Server {
	s.target = t
	return s
}

// Serve speaks the protocol on stdin/stdout until the client sends exit or hangs up.
func Serve(stdRoot string, target buildconfig.Target, cache string, o buildconfig.CompilerOptions, lazyStd bool) error {
	if o.Cwd == "" {
		return stderrors.New("lsp: CompilerOptions.Cwd is required; the caller fills it from the process")
	}
	s := New(stdRoot).WithTarget(target).WithPackageCache(cache, o).WithLazyStd(lazyStd)
	return s.serve(stdio{})
}

// serve returns right after exit without closing the connection: its reader may be parked on a stdin the client keeps open.
func (s *Server) serve(rwc io.ReadWriteCloser) error {
	conn := s.Connect(context.Background(), newRecoveringStream(rwc))
	select {
	case <-conn.Done():
		if err := conn.Err(); err != nil && err != io.EOF {
			return err
		}
		return nil
	case <-s.life.exited:
		return s.life.exitErr()
	}
}

func (s *Server) Shutdown(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil && s.timer.Stop() {
		s.work.Done()
	}
	for _, st := range s.states {
		if st.cancel != nil {
			st.cancel()
		}
	}
	return nil
}
func (s *Server) Exit(context.Context) error                               { return nil }
func (s *Server) SetTrace(context.Context, *protocol.SetTraceParams) error { return nil }
