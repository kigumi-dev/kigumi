package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// InlayHint renders inferred types for `let` bindings that carry no
// annotation, so the editor shows what the checker deduced.
func (s *Server) InlayHint(ctx context.Context, p *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.loadDocFresh(ctx, p.TextDocument.URI)
	if snap.res == nil {
		return nil, nil
	}
	t := snap.fileOf(uriToPath(p.TextDocument.URI))
	if t == nil {
		return nil, nil
	}
	info := snap.res.File(t)
	var out []protocol.InlayHint
	for v := range t.All() {
		if v.Kind() != syntax.LetStmt {
			continue
		}
		lv := v.AsLet()
		if lv.Type().ID() != 0 {
			continue
		}
		pat := lv.Pattern()
		if pat.Kind() != syntax.PatBind {
			continue
		}
		ent := info.Defs[pat.ID()]
		if ent == 0 {
			continue
		}
		e := snap.res.Entity(ent)
		if e.Type == 0 || e.Type == sem.TyPoison {
			continue
		}
		if e.Tok == 0 || int(e.Tok) >= len(t.Toks) {
			continue
		}
		label := snap.res.TypeString(e.Type)
		if label == "" || label == "?" {
			continue
		}
		pos := position(t.File, t.Toks[e.Tok].Span.End)
		if !rangeContains(p.Range, pos) {
			continue
		}
		out = append(out, protocol.InlayHint{Position: pos, Label: protocol.String(": " + label), Kind: protocol.InlayHintKindType})
	}
	return out, nil
}

// TypeDefinition jumps from a value to the declaration of its named type,
// looking through reference and pointer layers as gopls does.
func (s *Server) TypeDefinition(ctx context.Context, p *protocol.TypeDefinitionParams) (protocol.DefinitionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, t, n, ent := s.entityAt(ctx, p.TextDocumentPositionParams, false)
	if t == nil {
		return nil, nil
	}
	info := snap.res.File(t)
	ty := info.Types[n]
	if ent != 0 {
		if et := snap.res.Entity(ent).Type; et != 0 {
			ty = et
		}
	}
	for ty != 0 {
		nd := snap.res.Types.Node(ty)
		switch nd.Kind {
		case sem.KRef, sem.KPtr:
			ty = nd.Elem
		case sem.KNamed, sem.KIface, sem.KParam:
			if nd.Ent == 0 {
				return nil, nil
			}
			if loc := s.entityLocation(snap, nd.Ent); loc != nil {
				return loc, nil
			}
			return nil, nil
		default:
			return nil, nil
		}
	}
	return nil, nil
}

// rangeContains reports whether pos lies in r, end included so a hint at
// the end of a fully selected binding stays visible.
func rangeContains(r protocol.Range, pos protocol.Position) bool {
	return !positionBefore(pos, r.Start) && !positionBefore(r.End, pos)
}

func positionBefore(a, b protocol.Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Character < b.Character
}
