package lsp

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/syntax"
)

const maxWorkspaceSymbols = 100

// declSymbol classifies a top-level declaration; methods are named
// `Type.method`. ok is false for declarations without a name.
func declSymbol(t *syntax.Tree, d syntax.NodeID) (name string, kind protocol.SymbolKind, nameTok uint32, ok bool) {
	switch t.Kind(d) {
	case syntax.FnDecl:
		kind = protocol.SymbolKindFunction
	case syntax.TypeDecl:
		kind = protocol.SymbolKindStruct
	case syntax.InterfaceDecl:
		kind = protocol.SymbolKindInterface
	case syntax.ConstDecl:
		kind = protocol.SymbolKindConstant
	case syntax.StaticDecl:
		kind = protocol.SymbolKindVariable
	default:
		return "", 0, 0, false
	}
	decl := t.View(d).AsDecl()
	if !decl.Name().Valid() {
		return "", 0, 0, false
	}
	name, nameTok = decl.Name().Text(), decl.Name().Index()
	if t.Kind(d) == syntax.FnDecl {
		if recv := decl.AsFnDecl().Recv(); recv.Valid() {
			sp := recv.Span()
			name = string(t.File.Src[sp.Start:sp.End]) + "." + name
			kind = protocol.SymbolKindMethod
		}
	}
	return name, kind, nameTok, true
}

type memberSymbol struct {
	name string
	kind protocol.SymbolKind
	node syntax.NodeID
	tok  uint32
}

// memberSymbols lists the requirements of an interface and the fields of a
// record type, which sit one list below the declaration.
func memberSymbols(t *syntax.Tree, d syntax.NodeID) []memberSymbol {
	var out []memberSymbol
	for _, members := range memberLists(t, d) {
		for mv := range members.Views() {
			switch mv.Kind() {
			case syntax.FnDecl:
				if name := mv.AsDecl().Name(); name.Valid() {
					out = append(out, memberSymbol{name.Text(), protocol.SymbolKindMethod, mv.ID(), name.Index()})
				}
			case syntax.Field:
				if tok := mv.Tok(); tok.Valid() {
					out = append(out, memberSymbol{tok.Text(), protocol.SymbolKindField, mv.ID(), tok.Index()})
				}
			}
		}
	}
	return out
}

// memberLists is the list one level below a declaration that holds its
// members: an interface's requirements, a record or resource type's fields.
func memberLists(t *syntax.Tree, d syntax.NodeID) []syntax.ListView {
	v := t.View(d)
	var lists []syntax.ListView
	switch v.Kind() {
	case syntax.InterfaceDecl:
		lists = append(lists, v.AsInterfaceDecl().Members())
	case syntax.TypeDecl:
		if body := v.AsTypeDecl().Body(); body.Is(syntax.RecordBody, syntax.ResourceBody) {
			lists = append(lists, body.AsList())
		}
	}
	return lists
}

// Symbols searches the top-level declarations of every module in the
// workspace with a fuzzy, smart-case match; like gopls it answers an
// empty query with nothing and caps the result at 100.
func (s *Server) Symbols(ctx context.Context, p *protocol.WorkspaceSymbolParams) (protocol.WorkspaceSymbolResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := protocol.SymbolInformationSlice{}
	if p.Query == "" || len(s.roots) == 0 {
		return out, nil
	}
	for _, root := range s.allModuleRoots() {
		snap := s.load(ctx, root, "")
		if snap.mod == nil {
			continue
		}
		for _, path := range snap.mod.Order {
			pkg := snap.mod.Packages[path]
			if pkg.Std {
				continue
			}
			for _, fs := range s.packageSymbols(snap, path).files {
				u := uri.File(s.pathOfFile(snap, fs.file))
				for _, it := range fs.items {
					if !fuzzyMatch(p.Query, it.name) {
						continue
					}
					container := path
					out = append(out, protocol.SymbolInformation{BaseSymbolInformation: protocol.BaseSymbolInformation{Name: it.name, Kind: it.kind, ContainerName: &container}, Location: protocol.Location{URI: u, Range: it.rng}})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Name) != len(out[j].Name) {
			return len(out[i].Name) < len(out[j].Name)
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > maxWorkspaceSymbols {
		out = out[:maxWorkspaceSymbols]
	}
	return out, nil
}

// fuzzyMatch accepts name when query's characters appear in it in order;
// a query without upper-case letters matches case-insensitively.
func fuzzyMatch(query, name string) bool {
	if !strings.ContainsFunc(query, unicode.IsUpper) {
		query, name = strings.ToLower(query), strings.ToLower(name)
	}
	rest := name
	for _, q := range query {
		i := strings.IndexRune(rest, q)
		if i < 0 {
			return false
		}
		rest = rest[i+len(string(q)):]
	}
	return true
}
