package lsp

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

func (s *Server) publish(ctx context.Context, u uri.URI, ds []protocol.Diagnostic) {
	if ds == nil {
		ds = []protocol.Diagnostic{}
	}
	if client, ok := protocol.ClientFromContext(ctx); ok {
		p := &protocol.PublishDiagnosticsParams{URI: u, Diagnostics: ds}
		if v, ok := s.docVersions[u]; ok {
			p.Version = protocol.NewOptional(v)
		}
		client.PublishDiagnostics(ctx, p)
	}
}

func (s *Server) analyze(ctx context.Context, u uri.URI) {
	path := uriToPath(u)
	if module.IsManifest(filepath.Base(path)) {
		s.publish(ctx, u, s.manifestDiagnostics(u))
	}
	own := s.moduleRoot(path)
	for _, root := range s.invalidateFile(path) {
		hint := uri.URI("")
		if root == own {
			hint = u
		}
		s.analyzeModule(ctx, root, hint, true)
	}
}

// analyzeModule publishes root's diagnostics; force re-analyzes instead of
// reusing a cached snapshot.
func (s *Server) analyzeModule(ctx context.Context, root string, u uri.URI, force bool) {
	delete(s.pending, root)
	var snap *snapshot
	if force {
		snap = s.refresh(ctx, root, u, false)
	} else {
		snap = s.load(ctx, root, u)
	}
	s.publishSnapshot(ctx, snap, u)
}

// publishSnapshot sends the diagnostics of every own file of snap's module;
// a file that had diagnostics before and has none now gets an empty list.
func (s *Server) publishSnapshot(ctx context.Context, snap *snapshot, u uri.URI) {
	root := snap.root
	if snap.mod == nil {
		if u != "" && !module.IsManifest(filepath.Base(uriToPath(u))) {
			s.publish(ctx, u, []protocol.Diagnostic{{Message: protocol.String(snap.err.Error()), Severity: protocol.DiagnosticSeverityError}})
		}
		return
	}
	byFile := s.diagnosticsByURI(snap)
	fresh, stale := map[uri.URI]bool{}, s.published[root]
	sent := map[uri.URI]bool{}
	for _, fileURI := range byFile.order {
		ds := byFile.diags[fileURI]
		if len(ds) > 0 || stale[fileURI] || fileURI == u {
			s.publish(ctx, fileURI, ds)
			sent[fileURI] = true
		}
		fresh[fileURI] = len(ds) > 0
	}
	for d := range stale {
		if !fresh[d] && !sent[d] {
			s.publish(ctx, d, nil)
		}
	}
	s.published[root] = fresh
	s.logf(ctx, protocol.MessageTypeLog, "analyzed %s: %d packages, %d files with diagnostics", snap.root, len(snap.mod.Order), countTrue(fresh))
}

// pickEntry treats the one script-shaped file outside cmd/ as the entry,
// or the preferred (open) file when several qualify; the CLI stays strict.
func pickEntry(mod *driver.Module, prefer string) string {
	var found []string
	for _, p := range mod.Order {
		pkg := mod.Packages[p]
		if pkg.Std || strings.HasPrefix(mod.Rel(p), "cmd/") {
			continue
		}
		for _, t := range pkg.Files {
			if t.HasTopLevelStatements() || t.HasExplicitMain() {
				found = append(found, t.File.Name)
			}
		}
	}
	if len(found) == 1 {
		return found[0]
	}
	if slices.Contains(found, prefer) {
		return prefer
	}
	return ""
}

func countTrue(m map[uri.URI]bool) int {
	n := 0
	for _, v := range m {
		if v {
			n++
		}
	}
	return n
}
