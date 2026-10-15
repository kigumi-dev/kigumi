package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"kigumi/internal/driver"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// entityAt resolves the entity at a position; a stale snapshot answers unless fresh is set.
func (s *Server) entityAt(ctx context.Context, p protocol.TextDocumentPositionParams, fresh bool) (*snapshot, *syntax.Tree, syntax.NodeID, sem.EntityID) {
	load := s.loadDoc
	if fresh {
		load = s.loadDocFresh
	}
	snap := load(ctx, p.TextDocument.URI)
	if snap.mod == nil || snap.res == nil {
		return snap, nil, 0, 0
	}
	t := snap.fileOf(uriToPath(p.TextDocument.URI))
	if t == nil {
		return snap, nil, 0, 0
	}
	pos, ok := s.offsetIn(snap, t.File, p.Position)
	if !ok {
		return snap, nil, 0, 0
	}
	n := snap.posIndexOf(t).nodeAt(pos)
	if n == 0 {
		return snap, t, 0, snap.semIndex().declaredAt(t, pos)
	}
	info := snap.res.File(t)
	ent := info.Uses[n]
	if ent == 0 {
		ent = info.Defs[n]
	}
	return snap, t, n, ent
}

func (s *Server) Hover(ctx context.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, t, n, ent := s.entityAt(ctx, p.TextDocumentPositionParams, false)
	if t == nil || n == 0 && ent == 0 {
		return nil, nil
	}
	info := snap.res.File(t)
	text := ""
	switch {
	case ent != 0 && n != 0 && t.Kind(n) == syntax.MemberExpr && snap.res.Entity(ent).Kind == sem.EntType:
		text = definedValueHover(snap.res, ent, info.Types[n])
	case ent != 0:
		text = hoverText(snap.res, ent, info.Types[n])
	case info.Types[n] != 0:
		text = "```kigumi\n" + snap.res.TypeString(info.Types[n]) + "\n```"
	}
	if text == "" {
		return nil, nil
	}
	sp := t.Span(n)
	if n == 0 {
		sp = t.Toks[snap.res.Entity(ent).Tok].Span
	}
	r := s.rng(snap, t.File, sp)
	return &protocol.Hover{Contents: &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: text}, Range: &r}, nil
}

func (s *Server) Definition(ctx context.Context, p *protocol.DefinitionParams) (protocol.DefinitionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, t, _, ent := s.entityAt(ctx, p.TextDocumentPositionParams, false)
	if t == nil || ent == 0 {
		return nil, nil
	}
	if loc := s.entityLocation(snap, ent); loc != nil {
		return loc, nil
	}
	return nil, nil
}

// Formatting returns a format failure as an error, not an empty edit list,
// so the client can tell it apart from "already formatted".
func (s *Server) Formatting(ctx context.Context, p *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return nil, nil
	}
	out, err := driver.Format(driver.Source{Path: uriToPath(p.TextDocument.URI), Src: []byte(text)})
	if err != nil {
		return nil, err
	}
	if out == text {
		return []protocol.TextEdit{}, nil
	}
	return []protocol.TextEdit{{Range: protocol.Range{End: position(fileOfText(text), token.Pos(len(text)))}, NewText: out}}, nil
}
