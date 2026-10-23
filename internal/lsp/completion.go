package lsp

import (
	"context"
	"sort"

	"go.lsp.dev/protocol"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// memberCompletion answers a request right after a dot with the receiver's
// members only; ok is false without a member position or a known receiver.
func (s *Server) memberCompletion(ctx context.Context, p protocol.TextDocumentPositionParams) (protocol.CompletionItemSlice, bool) {
	text, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return nil, false
	}
	f := fileOfText(text)
	pos := offset(f, p.Position)
	dot := dotBefore(f.Src, pos)
	if dot < 0 {
		return nil, false
	}
	snap := s.loadDocFresh(ctx, p.TextDocument.URI)
	if snap.mod == nil || snap.res == nil {
		return nil, false
	}
	t := snap.fileOf(uriToPath(p.TextDocument.URI))
	if t == nil {
		return nil, false
	}
	recv := receiverAt(t, token.Pos(dot))
	if recv == 0 {
		return nil, false
	}
	info := snap.res.File(t)
	static := false
	if t.Kind(recv) == syntax.Ident {
		sp := t.Span(recv)
		if ent := importNamed(snap.res, scopeAt(snap, t, sp.Start), string(t.File.Src[sp.Start:sp.End])); ent != 0 {
			if items, ok := packageCompletion(snap.res, ent, info.Pkg); ok {
				return items, true
			}
		}
	}
	if ent := info.Uses[recv]; ent != 0 {
		switch snap.res.Entity(ent).Kind {
		case sem.EntType, sem.EntAlias:
			static = true
		}
	}
	rt := info.Types[recv]
	if static {
		rt = snap.res.Entity(info.Uses[recv]).Type
	}
	if rt == 0 {
		return nil, false
	}
	var items protocol.CompletionItemSlice
	for _, m := range snap.res.Members(rt, static) {
		e := snap.res.Entity(m.Ent)
		kind := protocol.CompletionItemKindMethod
		switch e.Kind {
		case sem.EntField:
			kind = protocol.CompletionItemKindField
		case sem.EntVariant:
			kind = protocol.CompletionItemKindEnumMember
		case sem.EntFn:
			if snap.res.Fn(m.Ent).Recv == sem.RecvNone {
				kind = protocol.CompletionItemKindFunction
			}
		}
		items = append(items, protocol.CompletionItem{Label: m.Name, Kind: kind, Detail: protocol.NewOptional(describe(snap.res, m.Ent, 0))})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items, true
}

// dotBefore is the offset of the `.` the cursor's identifier hangs on:
// the cursor sits right after it, or after the letters typed since.
func dotBefore(src []byte, pos token.Pos) int {
	i := int(pos)
	for i > 0 && isIdentByte(src[i-1]) {
		i--
	}
	if i > 0 && src[i-1] == '.' {
		return i - 1
	}
	return -1
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// receiverAt: the innermost node ending right before the dot wins, so
// `a.b.` names `a.b`.
func receiverAt(t *syntax.Tree, pos token.Pos) syntax.NodeID {
	best, bestLen := syntax.NodeID(0), -1
	for v := range t.All() {
		n := v.ID()
		switch v.Kind() {
		case syntax.Ident, syntax.MemberExpr, syntax.CallExpr, syntax.BracketExpr, syntax.Paren, syntax.StringLit, syntax.ByteStringLit, syntax.IntLit, syntax.FloatLit:
		default:
			continue
		}
		sp := v.Span()
		if v.Kind() == syntax.MemberExpr {
			// A member node's span covers only its name token; the whole
			// expression ends where the name does.
			m := v.AsMember()
			sp.Start = m.Base().Span().Start
			sp.End = m.Member().Span().End
		}
		if sp.End != pos || sp.End <= sp.Start {
			continue
		}
		if bestLen < 0 || sp.Len() < bestLen {
			best, bestLen = n, sp.Len()
		}
	}
	return best
}
