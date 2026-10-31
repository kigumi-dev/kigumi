package lsp

import (
	"context"
	"sync/atomic"

	"go.lsp.dev/uri"
)

// rootState is one module root's analysis generation: every edit that can
// change the root bumps gen, and a result built under an older gen is dropped.
type rootState struct {
	gen    uint64
	dirty  bool
	cancel context.CancelFunc
}

// analysisInput is everything build reads, copied out of the server so the
// analysis can run without s.mu.
type analysisInput struct {
	root    string
	hint    uri.URI
	path    string
	gen     uint64
	overlay map[string][]byte
	ctx     context.Context
	cancel  context.CancelFunc
}

// Counters tells how many analyses ran, were published, and were dropped as stale.
type Counters struct {
	Built, Installed, Dropped int64
	// SymbolPackages counts packages whose symbol table was rebuilt rather than reused by digest.
	SymbolPackages int64
}

type counters struct{ built, installed, dropped, symPkgs atomic.Int64 }

// Counters reports the analysis counts since the server started.
func (s *Server) Counters() Counters {
	return Counters{s.counts.built.Load(), s.counts.installed.Load(), s.counts.dropped.Load(), s.counts.symPkgs.Load()}
}

func (s *Server) state(root string) *rootState {
	st := s.states[root]
	if st == nil {
		st = &rootState{}
		s.states[root] = st
	}
	return st
}

// bump marks root as having unanalyzed edits and cancels its running analysis.
func (s *Server) bump(root string) {
	st := s.state(root)
	st.gen++
	st.dirty = true
	if st.cancel != nil {
		st.cancel()
		st.cancel = nil
	}
}

func (s *Server) begin(ctx context.Context, root string, u uri.URI) *analysisInput {
	st := s.state(root)
	overlay := make(map[string][]byte, len(s.docs))
	for d, text := range s.docs {
		overlay[uriToPath(d)] = []byte(text)
	}
	in := &analysisInput{root: root, hint: u, gen: st.gen, overlay: overlay}
	if u != "" {
		in.path = uriToPath(u)
	}
	in.ctx, in.cancel = context.WithCancel(ctx)
	st.cancel = in.cancel
	return in
}

// store caches snap unless an edit superseded the input it was built from.
func (s *Server) store(in *analysisInput, snap *snapshot) bool {
	st := s.state(in.root)
	if in.gen != st.gen || in.ctx.Err() != nil {
		s.counts.dropped.Add(1)
		return false
	}
	for key, old := range s.snaps {
		if old.root == in.root && old.gen < in.gen {
			delete(s.snaps, key)
		}
	}
	s.snaps[snapKey(in.root, in.hint)] = snap
	s.last[in.root] = snap
	st.dirty = false
	if snap.mod != nil {
		s.indexDeps(in.root, snap.mod)
	}
	s.counts.installed.Add(1)
	return true
}

// afterEdit analyzes inside the request when debouncing is off, else coalesces keystrokes.
func (s *Server) afterEdit(ctx context.Context, u uri.URI) {
	if s.debounce <= 0 {
		s.analyze(ctx, u)
		return
	}
	s.schedule(ctx, u)
}
