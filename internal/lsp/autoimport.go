package lsp

import (
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"kigumi/internal/driver"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// importIndex maps what a file could import: qualifiers to the paths they
// name, and public names to the paths declaring them.
type importIndex struct {
	qualifiers map[string][]string
	names      map[string][]namedDecl
}

type namedDecl struct {
	path string
	kind protocol.CompletionItemKind
}

func buildIndex(mod *driver.Module, current string) importIndex {
	ix := importIndex{qualifiers: map[string][]string{}, names: map[string][]namedDecl{}}
	paths := append([]string{}, mod.Order...)
	sort.SliceStable(paths, func(i, j int) bool { return sourceRank(mod.Packages[paths[i]]) < sourceRank(mod.Packages[paths[j]]) })
	for _, path := range paths {
		pkg := mod.Packages[path]
		if path == current || path == "std/prelude" || isEntryPackage(pkg) {
			continue
		}
		q := path[strings.LastIndex(path, "/")+1:]
		ix.qualifiers[q] = append(ix.qualifiers[q], path)
		for _, t := range pkg.Files {
			for d := range t.List(t.Root).IDs() {
				name, kind, tok, ok := declSymbol(t, d)
				if !ok || strings.Contains(name, ".") || !exported(t, d, pkg.Module == "" && !pkg.Std) {
					continue
				}
				ix.names[name] = append(ix.names[name], namedDecl{path: path, kind: completionKind(kind)})
				_ = tok
			}
		}
	}
	return ix
}

func sourceRank(p *driver.Package) int {
	switch {
	case p.Std:
		return 0
	case p.Module == "":
		return 1
	}
	return 2
}

func isEntryPackage(p *driver.Package) bool {
	for _, t := range p.Files {
		if t.HasTopLevelStatements() || t.HasExplicitMain() {
			return true
		}
	}
	return false
}

// exported accepts `pub` declarations, and `pub(module)` ones when the
// importing file is in the same module.
func exported(t *syntax.Tree, d syntax.NodeID, sameModule bool) bool {
	vis := t.View(d).AsDecl().Vis()
	if !vis.Valid() {
		return false
	}
	kind, _ := t.VisKind(vis.ID())
	return kind == "pub" || (kind == "module" && sameModule)
}

func completionKind(k protocol.SymbolKind) protocol.CompletionItemKind {
	switch k {
	case protocol.SymbolKindFunction:
		return protocol.CompletionItemKindFunction
	case protocol.SymbolKindInterface:
		return protocol.CompletionItemKindInterface
	case protocol.SymbolKindConstant:
		return protocol.CompletionItemKindConstant
	}
	return protocol.CompletionItemKindClass
}

func importEdit(t *syntax.Tree, path, name string) protocol.TextEdit {
	var lastImport syntax.NodeID
	for dv := range t.List(t.Root).Views() {
		if dv.Kind() != syntax.ImportDecl {
			continue
		}
		d := dv.AsImportDecl()
		lastImport = d.ID()
		if binding := d.Binding(); name != "" && d.Path().Join("/") == path && binding.Kind() == syntax.List {
			items := []importItem{{key: name, text: name}}
			for nm := range binding.AsList().Views() {
				items = append(items, importItem{key: nm.Tok().Text(), text: importItemText(nm)})
			}
			sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })
			names := make([]string, len(items))
			for i, it := range items {
				names[i] = it.text
			}
			sp := binding.Span()
			sp.End++ // the closing brace is not a node
			return protocol.TextEdit{Range: spanRange(t.File, sp), NewText: "{" + strings.Join(names, ", ") + "}"}
		}
	}
	line := "import " + path[strings.LastIndex(path, "/")+1:] + " from " + path + "\n"
	if name != "" {
		line = "import {" + name + "} from " + path + "\n"
	}
	var at token.Pos
	switch stmts := t.List(t.Root); {
	case lastImport != 0:
		at = token.Pos(lineEndOf(t.File.Src, int(t.Span(lastImport).End)))
	case stmts.Len() > 0:
		at = token.Pos(lineStartOf(t.File.Src, int(stmts.At(0).Span().Start)))
		line += "\n"
	default:
		at = token.Pos(len(t.File.Src))
	}
	return protocol.TextEdit{Range: spanRange(t.File, token.Span{Start: at, End: at}), NewText: line}
}

// importItem is one `{}` binding item sorted by its local (bound) name;
// key and text differ only for a renamed import (`orig as alias`).
type importItem struct{ key, text string }

func importItemText(nm syntax.View) string {
	if nm.Kind() != syntax.ImportAlias {
		return nm.Tok().Text()
	}
	a := nm.AsImportAlias()
	return a.Name().Name().Text() + " as " + a.Alias().Name().Text()
}

func lineEndOf(src []byte, pos int) int {
	for pos < len(src) && src[pos-1] != '\n' {
		pos++
	}
	return pos
}

func lineStartOf(src []byte, pos int) int {
	for pos > 0 && src[pos-1] != '\n' {
		pos--
	}
	return pos
}

// boundNames lists the qualifiers and selected names a file already
// imports, so completion does not offer to import them again.
func boundNames(t *syntax.Tree) map[string]bool {
	out := map[string]bool{}
	for dv := range t.List(t.Root).Views() {
		if dv.Kind() != syntax.ImportDecl {
			continue
		}
		b := dv.AsImportDecl().Binding()
		if b.Kind() == syntax.List {
			for nm := range b.AsList().Views() {
				out[nm.Tok().Text()] = true
			}
			continue
		}
		out[b.Tok().Text()] = true
	}
	return out
}
