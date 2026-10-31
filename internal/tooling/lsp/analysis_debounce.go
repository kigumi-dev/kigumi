package lsp

import (
	"context"
	"path/filepath"
	"time"

	"go.lsp.dev/uri"

	"kigumi/internal/toolchain/driver"
)

// schedule invalidates the roots path belongs to and re-arms the debounce
// timer; the analysis itself runs on the timer's goroutine.
func (s *Server) schedule(ctx context.Context, u uri.URI) {
	path := uriToPath(u)
	if driver.IsManifest(filepath.Base(path)) {
		s.publish(ctx, u, s.manifestDiagnostics(u))
	}
	s.bg = context.WithoutCancel(ctx)
	own := s.moduleRoot(path)
	for _, root := range s.invalidateFile(path) {
		hint := uri.URI("")
		if root == own {
			hint = u
		}
		s.pending[root] = hint
	}
	if s.timer != nil && s.timer.Stop() {
		s.work.Done()
	}
	s.work.Add(1)
	s.timer = time.AfterFunc(s.debounce, s.flush)
}

// flush analyzes the pending roots; a result is installed and published
// only when no edit arrived while it ran.
func (s *Server) flush() {
	defer s.work.Done()
	s.mu.Lock()
	bg := s.bg
	jobs := s.pending
	s.pending = map[string]uri.URI{}
	ins := make([]*analysisInput, 0, len(jobs))
	for root, hint := range jobs {
		ins = append(ins, s.begin(bg, root, hint))
	}
	s.mu.Unlock()
	for _, in := range ins {
		snap := s.build(bg, in)
		s.mu.Lock()
		if s.store(in, snap) {
			s.publishSnapshot(bg, snap, in.hint)
		}
		s.mu.Unlock()
		in.cancel()
	}
}

// WaitIdle blocks until no debounce timer is armed and no background analysis is running.
func (s *Server) WaitIdle() { s.work.Wait() }
