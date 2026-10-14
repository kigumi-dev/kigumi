package lsp

import (
	"context"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/driver"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// snapshot is immutable once cached under a key: it is replaced, never mutated.
type snapshot struct {
	root    string
	mod     *driver.Module
	res     *sem.Result
	err     error
	origins map[string]fileOrigin
}

func (s *Server) loadDoc(ctx context.Context, u uri.URI) *snapshot {
	return s.load(ctx, s.moduleRoot(uriToPath(u)), u)
}

func (s *Server) load(ctx context.Context, root string, u uri.URI) *snapshot {
	path := ""
	if u != "" {
		path = uriToPath(u)
	}
	key := root + "\x00" + path
	if snap, ok := s.snaps[key]; ok {
		return snap
	}
	snap := s.loadFresh(ctx, root, u, path)
	s.snaps[key] = snap
	return snap
}

func (s *Server) loadFresh(ctx context.Context, root string, u uri.URI, path string) *snapshot {
	overlay := map[string][]byte{}
	for d, text := range s.docs {
		overlay[uriToPath(d)] = []byte(text)
	}
	mod, err := driver.LoadModule(root, driver.LoadOptions{Test: true, StdRoot: s.stdRoot, Overlay: overlay, Target: s.target})
	if err == nil && mod.Entry == "" {
		prefer, _ := filepath.Rel(root, path)
		mod.Entry = pickEntry(mod, filepath.ToSlash(prefer))
	}
	snap := &snapshot{root: root, mod: mod, err: err}
	if err != nil {
		s.logf(ctx, protocol.MessageTypeError, "load module %s: %v", root, err)
		return snap
	}
	snap.origins = buildOrigins(mod)
	s.indexDeps(root, mod)
	// Edited files rarely fully parse; recovering here keeps hover,
	// completion and definition working around the broken statement.
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				snap.res = nil
				s.logf(ctx, protocol.MessageTypeError, "checker panicked on %s: %v", path, rec)
			}
		}()
		snap.res, snap.err = driver.CheckForEditor(mod)
	}()
	if snap.err != nil {
		s.logf(ctx, protocol.MessageTypeError, "check %s: %v", root, snap.err)
	}
	return snap
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
