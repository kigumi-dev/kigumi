package lsp

import (
	"context"
	"sort"

	"go.lsp.dev/protocol"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// semanticTokenTypes is the legend SemanticTokensFull encodes tokenType
// indices against; order is the wire contract, so it only ever grows.
var semanticTokenTypes = []string{
	"namespace", "type", "interface", "typeParameter", "parameter",
	"variable", "property", "enumMember", "function", "method",
}

var semanticTokenModifiers = []string{"declaration", "readonly", "defaultLibrary"}

const (
	modDeclaration = 1 << iota
	modReadonly
	modDefaultLibrary
)

type semanticToken struct {
	span      token.Span
	typ, mods uint32
}

// SemanticTokensFull answers from the checker's entity table, so a local
// shadowing a std function is colored as the local, not the grammar's guess.
func (s *Server) SemanticTokensFull(ctx context.Context, p *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.loadDoc(ctx, p.TextDocument.URI)
	if snap.res == nil {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}
	t := snap.fileOf(uriToPath(p.TextDocument.URI))
	if t == nil {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}
	info := snap.res.File(t)
	var items []semanticToken
	add := func(n int, ent sem.EntityID, declared bool) {
		if ent == 0 || n == 0 {
			return
		}
		typ, mods, ok := semanticTokenFor(snap.res, ent)
		if !ok {
			return
		}
		if declared {
			mods |= modDeclaration
		}
		items = append(items, semanticToken{nameSpan(t, syntax.NodeID(n)), typ, mods})
	}
	for n, ent := range info.Defs {
		add(n, ent, true)
	}
	for n, ent := range info.Uses {
		add(n, ent, false)
	}
	return &protocol.SemanticTokens{Data: encodeSemanticTokens(t.File, items)}, nil
}

// encodeSemanticTokens sorts by position and applies the spec's delta
// encoding: five uint32s per token (deltaLine, deltaStartChar, length, type, mods).
func encodeSemanticTokens(f *token.File, items []semanticToken) []uint32 {
	sort.Slice(items, func(i, j int) bool { return items[i].span.Start < items[j].span.Start })
	data := make([]uint32, 0, len(items)*5)
	prevLine, prevChar := uint32(0), uint32(0)
	for _, it := range items {
		start := position(f, it.span.Start)
		length := uint32(it.span.Len())
		deltaLine := start.Line - prevLine
		deltaChar := start.Character
		if deltaLine == 0 {
			deltaChar = start.Character - prevChar
		}
		data = append(data, deltaLine, deltaChar, length, it.typ, it.mods)
		prevLine, prevChar = start.Line, start.Character
	}
	return data
}

// semanticTokenFor classifies an entity for the legend above; ok is false
// for kinds with no distinct coloring, leaving the grammar's guess in place.
func semanticTokenFor(res *sem.Result, ent sem.EntityID) (typ, mods uint32, ok bool) {
	e := res.Entity(ent)
	if e.Flags&sem.EfStd != 0 {
		mods |= modDefaultLibrary
	}
	switch e.Kind {
	case sem.EntPackage, sem.EntImport:
		return 0, mods, true
	case sem.EntType, sem.EntAlias:
		return 1, mods, true
	case sem.EntInterface, sem.EntConstraint:
		return 2, mods, true
	case sem.EntTypeParam:
		return 3, mods, true
	case sem.EntParam:
		return 4, mods, true
	case sem.EntConst:
		return 5, mods | modReadonly, true
	case sem.EntStatic:
		if !res.Static(ent).Mut {
			mods |= modReadonly
		}
		return 5, mods, true
	case sem.EntField:
		return 6, mods, true
	case sem.EntVariant:
		return 7, mods | modReadonly, true
	case sem.EntFn, sem.EntClosure:
		if e.Kind == sem.EntFn && res.Fn(ent).Owner != 0 {
			return 9, mods, true
		}
		return 8, mods, true
	default:
		return 0, 0, false
	}
}
