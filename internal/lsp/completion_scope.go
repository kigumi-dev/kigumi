package lsp

import (
	"go.lsp.dev/protocol"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// packageCompletion lists the members of the package an import binding names.
func packageCompletion(res *sem.Result, ent sem.EntityID, from sem.PackageID) (protocol.CompletionItemSlice, bool) {
	e := res.Entity(ent)
	if e.Kind != sem.EntImport || e.Target == 0 || res.Entity(e.Target).Kind != sem.EntPackage {
		return nil, false
	}
	items := protocol.CompletionItemSlice{}
	for _, m := range res.PackageMembers(res.Entity(e.Target).Pkg, from) {
		items = append(items, protocol.CompletionItem{Label: m.Name, Kind: completionKindOf(res.Entity(m.Ent).Kind), Detail: protocol.NewOptional(describe(res, m.Ent, 0))})
	}
	return items, true
}

// scopeItems lists the names visible at the position, innermost scope
// first, each with the kind and type the checker knows.
func (s *Server) scopeItems(snap *snapshot, p protocol.TextDocumentPositionParams) []protocol.CompletionItem {
	t := snap.fileOf(uriToPath(p.TextDocument.URI))
	if t == nil {
		return nil
	}
	scope := scopeAt(snap, t, offset(t.File, p.Position))
	var items []protocol.CompletionItem
	seen := map[string]bool{}
	for sc := scope; sc != 0; sc = snap.res.Scopes[sc].Parent {
		for name, b := range snap.res.Scopes[sc].Names {
			if seen[name] || name == "" || name[0] == '$' {
				continue
			}
			seen[name] = true
			item := protocol.CompletionItem{Label: name, Kind: protocol.CompletionItemKindVariable}
			ent := b.Ent
			if ent == 0 && b.Set != 0 && len(snap.res.Overloads[b.Set].Members) > 0 {
				ent = snap.res.Overloads[b.Set].Members[0]
			}
			if ent != 0 {
				item.Kind = completionKindOf(snap.res.Entity(ent).Kind)
				item.Detail = protocol.NewOptional(describe(snap.res, ent, 0))
			}
			items = append(items, item)
		}
	}
	return items
}

// scopeAt is the innermost scope covering pos.
func scopeAt(snap *snapshot, t *syntax.Tree, pos token.Pos) sem.ScopeID {
	info := snap.res.File(t)
	var scope sem.ScopeID
	bestLen := -1
	for v := range t.All() {
		n := v.ID()
		sc := info.Scopes[n]
		if sc == 0 {
			continue
		}
		sp := t.Span(n)
		if pos < sp.Start || pos > sp.End {
			continue
		}
		if bestLen < 0 || sp.Len() < bestLen {
			scope, bestLen = sc, sp.Len()
		}
	}
	if scope == 0 {
		scope = info.Scopes[t.Root]
	}
	return scope
}

// importNamed finds the binding that name resolves to at scope.
func importNamed(res *sem.Result, scope sem.ScopeID, name string) sem.EntityID {
	for sc := scope; sc != 0; sc = res.Scopes[sc].Parent {
		if b, ok := res.Scopes[sc].Names[name]; ok {
			return b.Ent
		}
	}
	return 0
}

func completionKindOf(k sem.EntityKind) protocol.CompletionItemKind {
	switch k {
	case sem.EntFn:
		return protocol.CompletionItemKindFunction
	case sem.EntType, sem.EntAlias:
		return protocol.CompletionItemKindClass
	case sem.EntInterface:
		return protocol.CompletionItemKindInterface
	case sem.EntConst:
		return protocol.CompletionItemKindConstant
	case sem.EntStatic:
		return protocol.CompletionItemKindVariable
	case sem.EntVariant:
		return protocol.CompletionItemKindEnumMember
	case sem.EntPackage, sem.EntImport:
		return protocol.CompletionItemKindModule
	}
	return protocol.CompletionItemKindVariable
}
