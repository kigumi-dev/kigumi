package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// Initialized analyzes the whole workspace so every file gets diagnostics
// before it is opened.
func (s *Server) Initialized(ctx context.Context, _ *protocol.InitializedParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.analyzeWorkspace(ctx)
	return nil
}

// DidChangeWatchedFiles re-analyzes only the modules a changed path
// belongs to or depends on, not the whole workspace.
func (s *Server) DidChangeWatchedFiles(ctx context.Context, p *protocol.DidChangeWatchedFilesParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	roots := map[string]bool{}
	for _, c := range p.Changes {
		for _, root := range s.invalidateFile(uriToPath(c.URI)) {
			roots[root] = true
		}
	}
	for root := range roots {
		s.analyzeModule(ctx, root, "")
	}
	return nil
}

func (s *Server) analyzeWorkspace(ctx context.Context) {
	for _, root := range s.allModuleRoots() {
		if ctx.Err() != nil {
			return
		}
		s.analyzeModule(ctx, root, "")
	}
}

func (s *Server) DidOpen(ctx context.Context, p *protocol.DidOpenTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	s.docVersions[p.TextDocument.URI] = p.TextDocument.Version
	s.analyze(ctx, p.TextDocument.URI)
	return nil
}

func (s *Server) DidChange(ctx context.Context, p *protocol.DidChangeTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := p.TextDocument.URI
	s.docVersions[u] = p.TextDocument.Version
	for _, change := range p.ContentChanges {
		switch c := change.(type) {
		case *protocol.TextDocumentContentChangeWholeDocument:
			s.docs[u] = c.Text
		case *protocol.TextDocumentContentChangePartial:
			text := s.docs[u]
			f := fileOfText(text)
			start, end := offset(f, c.Range.Start), offset(f, c.Range.End)
			s.docs[u] = text[:start] + c.Text + text[end:]
		}
	}
	s.analyze(ctx, u)
	return nil
}

func (s *Server) DidClose(ctx context.Context, p *protocol.DidCloseTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, p.TextDocument.URI)
	delete(s.docVersions, p.TextDocument.URI)
	s.analyze(ctx, p.TextDocument.URI)
	return nil
}
