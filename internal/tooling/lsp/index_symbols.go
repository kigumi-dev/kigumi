package lsp

import (
	"go.lsp.dev/protocol"

	"kigumi/internal/compiler/syntax"
)

// docSymbols is the outline of one open document's text.
type docSymbols struct {
	text string
	syms protocol.DocumentSymbolSlice
}

func documentSymbols(t *syntax.Tree) protocol.DocumentSymbolSlice {
	out := protocol.DocumentSymbolSlice{}
	for d := range t.List(t.Root).IDs() {
		name, kind, tok, ok := declSymbol(t, d)
		if !ok {
			continue
		}
		sym := protocol.DocumentSymbol{Name: name, Kind: kind, Range: declRange(t, d, tok), SelectionRange: spanRange(t.File, t.Toks[tok].Span)}
		for _, m := range memberSymbols(t, d) {
			sym.Children = append(sym.Children, protocol.DocumentSymbol{Name: m.name, Kind: m.kind, Range: declRange(t, m.node, m.tok), SelectionRange: spanRange(t.File, t.Toks[m.tok].Span)})
		}
		out = append(out, sym)
	}
	return out
}

type symItem struct {
	name string
	kind protocol.SymbolKind
	rng  protocol.Range
}

type fileSymbols struct {
	file  string
	items []symItem
}

// pkgSymbols lists the declarations of one package; digest is the package
// content it was built from, so an edit elsewhere leaves it valid.
type pkgSymbols struct {
	digest string
	files  []fileSymbols
}

// packageSymbols returns the symbol table of a package of snap, rebuilding
// it only when the package's content changed since the cached one.
func (s *Server) packageSymbols(snap *snapshot, path string) pkgSymbols {
	key := snap.root + "\x00" + path
	digest := snap.digests[path]
	if cached, ok := s.pkgSyms[key]; ok && cached.digest == digest {
		return cached
	}
	s.counts.symPkgs.Add(1)
	ps := pkgSymbols{digest: digest}
	for _, t := range snap.mod.Packages[path].Files {
		fs := fileSymbols{file: t.File.Name}
		for d := range t.List(t.Root).IDs() {
			name, kind, tok, ok := declSymbol(t, d)
			if !ok {
				continue
			}
			fs.items = append(fs.items, symItem{name, kind, spanRange(t.File, t.Toks[tok].Span)})
			for _, m := range memberSymbols(t, d) {
				fs.items = append(fs.items, symItem{name + "." + m.name, m.kind, spanRange(t.File, t.Toks[m.tok].Span)})
			}
		}
		ps.files = append(ps.files, fs)
	}
	s.pkgSyms[key] = ps
	return ps
}
