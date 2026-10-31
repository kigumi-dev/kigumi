package lsp

import (
	"context"
	"slices"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// entityLocation points at the declaring name of an entity; nil for
// entities without source (predeclared types, intrinsics).
func (s *Server) entityLocation(snap *snapshot, ent sem.EntityID) *protocol.Location {
	e := snap.res.Entity(ent)
	if e.File == 0 {
		return nil
	}
	target := snap.res.Tree(e.File)
	if target.File.Generated {
		return nil
	}
	sp := target.Span(e.Node)
	if e.Tok != 0 {
		sp = target.Toks[e.Tok].Span
	}
	return &protocol.Location{URI: uri.File(s.pathOfFile(snap, target.File.Name)), Range: s.rng(snap, target.File, sp)}
}

// nameSpan is the span of the name a use node refers with: the member
// token of a member expression, the path of a type, the whole node otherwise.
func nameSpan(t *syntax.Tree, n syntax.NodeID) token.Span {
	switch t.Kind(n) {
	case syntax.MemberExpr:
		return t.View(n).Tok().Span()
	case syntax.TypePath:
		return t.View(n).AsTypePath().Path().Span()
	}
	return t.Span(n)
}

// References lists every use of the entity (and, for methods, of the
// interface requirements or witnesses it corresponds to, as gopls does).
func (s *Server) References(ctx context.Context, p *protocol.ReferenceParams) ([]protocol.Location, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, t, _, ent := s.entityAt(ctx, p.TextDocumentPositionParams, false)
	if t == nil || ent == 0 {
		return nil, nil
	}
	targets := []sem.EntityID{ent}
	if isMethod(snap.res, ent) {
		targets = append(targets, snap.implementations(ent)...)
	}
	slices.Sort(targets)
	targets = slices.Compact(targets)
	out := []protocol.Location{}
	if p.Context.IncludeDeclaration {
		for _, id := range targets {
			if loc := s.entityLocation(snap, id); loc != nil {
				out = append(out, *loc)
			}
		}
	}
	for _, u := range snap.semIndex().usesOf(targets) {
		out = append(out, protocol.Location{URI: uri.File(s.pathOfFile(snap, u.tree.File.Name)), Range: s.rng(snap, u.tree.File, nameSpan(u.tree, u.node))})
	}
	return out, nil
}

// Implementation follows gopls: an interface leads to the types
// conforming to it, a type to the interfaces it conforms to, and a method
// to its counterparts on the other side.
func (s *Server) Implementation(ctx context.Context, p *protocol.ImplementationParams) (protocol.DefinitionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, t, _, ent := s.entityAt(ctx, p.TextDocumentPositionParams, false)
	out := protocol.LocationSlice{}
	if t == nil || ent == 0 {
		return out, nil
	}
	for _, target := range snap.implementations(ent) {
		if loc := s.entityLocation(snap, target); loc != nil {
			out = append(out, *loc)
		}
	}
	return out, nil
}

func isMethod(res *sem.Result, ent sem.EntityID) bool {
	return res.Entity(ent).Kind == sem.EntFn && res.Fn(ent).Owner != 0
}

func conforms(res *sem.Result, ty, iface sem.EntityID) ([]sem.EntityID, bool) {
	return res.Implements(res.SelfType(ty), res.SelfType(iface), res.Entity(ty).Pkg)
}

// declared lists the source-declared entities of one kind.
func declared(res *sem.Result, kind sem.EntityKind) []sem.EntityID {
	var out []sem.EntityID
	for i := range res.Entities {
		if e := &res.Entities[i]; e.Kind == kind && e.File != 0 {
			out = append(out, sem.EntityID(i))
		}
	}
	return out
}
