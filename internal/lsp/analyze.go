package lsp

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/diag"
	"kigumi/internal/driver"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// snapshot is immutable once cached under a key: it is replaced, never mutated.
type snapshot struct {
	root     string
	mod      *driver.Module
	res      *sem.Result
	err      error
	origins  map[string]fileOrigin
	gen      uint64
	digests  map[string]string
	idxOnce  sync.Once
	idx      *semIndex
	confOnce sync.Once
	conf     *conformIndex
	posMu    sync.Mutex
	pos      map[*syntax.Tree]*posIndex
	diagOnce sync.Once
	diags    map[string][]diag.Diagnostic
}

// loadDoc answers from the last complete snapshot while a newer analysis is pending.
func (s *Server) loadDoc(ctx context.Context, u uri.URI) *snapshot {
	return s.load(ctx, s.moduleRoot(uriToPath(u)), u)
}

// loadDocFresh analyzes now when the root has unanalyzed edits.
func (s *Server) loadDocFresh(ctx context.Context, u uri.URI) *snapshot {
	root := s.moduleRoot(uriToPath(u))
	if st := s.states[root]; st != nil && st.dirty {
		return s.refresh(ctx, root, u, false)
	}
	return s.load(ctx, root, u)
}

func (s *Server) load(ctx context.Context, root string, u uri.URI) *snapshot {
	key := snapKey(root, u)
	if snap, ok := s.snaps[key]; ok {
		return snap
	}
	if st := s.states[root]; st != nil && st.dirty && s.last[root] != nil {
		return s.last[root]
	}
	return s.refresh(ctx, root, u, false)
}

func snapKey(root string, u uri.URI) string {
	path := ""
	if u != "" {
		path = uriToPath(u)
	}
	return root + "\x00" + path
}

func (s *Server) refresh(ctx context.Context, root string, u uri.URI, bump bool) *snapshot {
	if bump {
		s.bump(root)
	}
	in := s.begin(ctx, root, u)
	snap := s.build(ctx, in)
	s.store(in, snap)
	in.cancel()
	return snap
}

// build reads only in, so it runs without s.mu.
func (s *Server) build(ctx context.Context, in *analysisInput) *snapshot {
	s.counts.built.Add(1)
	snap := &snapshot{root: in.root, gen: in.gen, pos: map[*syntax.Tree]*posIndex{}}
	mod, err := driver.LoadModule(in.root, driver.LoadOptions{Test: true, StdRoot: s.stdRoot, Overlay: in.overlay, Target: s.target, AllPackages: true, DeferParse: true, PackageCache: s.cache, Compiler: s.compiler})
	if err == nil && in.ctx.Err() != nil {
		err = in.ctx.Err()
	}
	if err == nil {
		err = mod.EnsureParsed()
	}
	snap.mod, snap.err = mod, err
	if err != nil {
		snap.mod = nil
		if in.ctx.Err() == nil {
			s.logf(ctx, protocol.MessageTypeError, "load module %s: %v", in.root, err)
		}
		return snap
	}
	if mod.Entry == "" {
		prefer, _ := filepath.Rel(in.root, in.path)
		mod.Entry = pickEntry(mod, filepath.ToSlash(prefer))
	}
	s.stage("loaded", in)
	snap.origins = buildOrigins(mod)
	snap.digests = map[string]string{}
	for _, p := range mod.Order {
		snap.digests[p] = mod.PackageDigest(p)
	}
	// Edited files rarely fully parse; recovering here keeps hover,
	// completion and definition working around the broken statement.
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				snap.res = nil
				s.logf(ctx, protocol.MessageTypeError, "checker panicked on %s: %v", in.path, rec)
			}
		}()
		snap.res, snap.err = driver.CheckForEditorContext(in.ctx, mod)
	}()
	if snap.err != nil && in.ctx.Err() == nil {
		s.logf(ctx, protocol.MessageTypeError, "check %s: %v", in.root, snap.err)
	}
	s.stage("checked", in)
	return snap
}

// stage lets a test pause or observe an analysis between load and check.
func (s *Server) stage(name string, in *analysisInput) {
	if s.hook != nil {
		s.hook(name, in)
	}
}

// indexDeps records which package directories root's snapshot read, so a
// later change under one of them invalidates root too, not just the owner.
func (s *Server) indexDeps(root string, mod *driver.Module) {
	for _, p := range mod.Order {
		dir := mod.Packages[p].Dir
		if s.depIndex[dir] == nil {
			s.depIndex[dir] = map[string]bool{}
		}
		s.depIndex[dir][root] = true
	}
}

// invalidateFile drops path's module and any root that last read its
// directory, root-grained since driver's checker isn't itself incremental.
func (s *Server) invalidateFile(path string) []string {
	roots := map[string]bool{s.moduleRoot(path): true}
	for root := range s.depIndex[filepath.Dir(path)] {
		roots[root] = true
	}
	for root := range roots {
		s.bump(root)
	}
	for key := range s.snaps {
		if root, _, ok := strings.Cut(key, "\x00"); ok && roots[root] {
			delete(s.snaps, key)
		}
	}
	out := make([]string, 0, len(roots))
	for root := range roots {
		out = append(out, root)
	}
	return out
}

func (snap *snapshot) fileOf(path string) *syntax.Tree {
	rel, err := filepath.Rel(snap.root, path)
	if err != nil {
		return nil
	}
	for _, p := range snap.mod.Packages {
		for _, t := range p.Files {
			if t.File.Name == filepath.ToSlash(rel) {
				return t
			}
		}
	}
	return nil
}

// pathOfFile resolves a file's logical name via the origin map; the join
// fallback is only for names buildOrigins never saw (a synthetic file).
func (s *Server) pathOfFile(snap *snapshot, name string) string {
	if o, ok := snap.originOf(name); ok {
		return o.path
	}
	if rest, ok := strings.CutPrefix(name, "std/"); ok {
		return filepath.Join(s.stdRoot, filepath.FromSlash(rest))
	}
	return filepath.Join(snap.root, filepath.FromSlash(name))
}
