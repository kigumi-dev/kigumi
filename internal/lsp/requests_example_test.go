package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		os.MkdirAll(filepath.Dir(dst), 0o755)
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// pos finds the nth (zero-based) occurrence of needle in text as an LSP position, offset chars into it.
func pos(t *testing.T, text, needle string, nth, offset int) protocol.Position {
	t.Helper()
	at := -1
	for i := 0; i <= nth; i++ {
		next := strings.Index(text[at+1:], needle)
		if next < 0 {
			t.Fatalf("%q occurs fewer than %d times", needle, nth+1)
		}
		at += next + 1
	}
	at += offset
	line := strings.Count(text[:at], "\n")
	return protocol.Position{Line: uint32(line), Character: uint32(at - (strings.LastIndex(text[:at], "\n") + 1))}
}

func at(u uri.URI, p protocol.Position) protocol.TextDocumentPositionParams {
	return protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}, Position: p}
}

func hasLine(list protocol.LocationSlice, u uri.URI, line uint32) bool {
	for _, l := range list {
		if l.URI == u && l.Range.Start.Line == line {
			return true
		}
	}
	return false
}

func TestRequestsOnTheRepostatExample(t *testing.T) {
	root := t.TempDir()
	copyTree(t, "../../examples/repostat", root)
	s, ctx, _ := startServer(t, root, 0)
	read := func(rel string) (uri.URI, string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		b, _ := os.ReadFile(p)
		return uri.File(p), string(b)
	}
	report, reportSrc := read("report/report.kg")
	parse, parseSrc := read("parse/parse.kg")
	openDoc(s, ctx, report, 1, reportSrc)
	openDoc(s, ctx, parse, 1, parseSrc)

	t.Run("definition crosses packages and reaches std", func(t *testing.T) {
		def, _ := s.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: at(report, pos(t, reportSrc, "tally(commits", 0, 2))})
		if loc, ok := def.(*protocol.Location); !ok || loc.URI != parse || loc.Range.Start.Line != 41 {
			t.Fatalf("definition of tally: %+v", def)
		}
		def, _ = s.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: at(report, pos(t, reportSrc, "array.sortedBy", 0, 8))})
		std, _ := filepath.Abs("../../std")
		if loc, ok := def.(*protocol.Location); !ok || !strings.HasPrefix(string(loc.URI), string(uri.File(std))) {
			t.Fatalf("definition of a std function must land in std: %+v", def)
		}
	})
	t.Run("hover shows the std signature", func(t *testing.T) {
		h, _ := s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: at(report, pos(t, reportSrc, "array.sortedBy", 0, 8))})
		if h == nil || !strings.Contains(h.Contents.(*protocol.MarkupContent).Value, "sortedBy") {
			t.Fatalf("hover: %+v", h)
		}
	})
	t.Run("references span files", func(t *testing.T) {
		refs, _ := s.References(ctx, &protocol.ReferenceParams{TextDocumentPositionParams: at(parse, pos(t, parseSrc, "fn tally", 0, 7)), Context: protocol.ReferenceContext{IncludeDeclaration: true}})
		files := map[uri.URI]int{}
		for _, r := range refs {
			files[r.URI]++
		}
		if files[parse] != 1 || files[report] != 2 {
			t.Fatalf("references of tally by file: %v", files)
		}
	})
	t.Run("implementation goes both ways", func(t *testing.T) {
		impl, _ := s.Implementation(ctx, &protocol.ImplementationParams{TextDocumentPositionParams: at(report, pos(t, reportSrc, "interface Display", 0, 12))})
		if !hasLine(impl.(protocol.LocationSlice), report, 11) {
			t.Fatalf("implementers of Display: %+v", impl)
		}
		back, _ := s.Implementation(ctx, &protocol.ImplementationParams{TextDocumentPositionParams: at(report, pos(t, reportSrc, "type AuthorStat", 0, 6))})
		if !hasLine(back.(protocol.LocationSlice), report, 6) {
			t.Fatalf("interfaces of AuthorStat: %+v", back)
		}
	})
	t.Run("document and workspace symbols", func(t *testing.T) {
		syms, _ := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: report}})
		names := map[string]bool{}
		for _, sym := range syms.(protocol.DocumentSymbolSlice) {
			names[sym.Name] = true
		}
		if !names["Display"] || !names["AuthorStat"] || !names["build"] {
			t.Fatalf("document symbols: %v", names)
		}
		ws, _ := s.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: "tally"})
		if list := ws.(protocol.SymbolInformationSlice); len(list) != 1 || list[0].Location.URI != parse || *list[0].ContainerName != "repostat/parse" {
			t.Fatalf("workspace symbols: %+v", ws)
		}
	})
}

func TestRequestsOnAStdFile(t *testing.T) {
	root := t.TempDir()
	copyTree(t, "../../examples/hello", root)
	s, ctx, _ := startServer(t, root, 0)
	std, _ := filepath.Abs("../../std")
	file := filepath.Join(std, "array", "array.kg")
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	u := uri.File(file)
	openDoc(s, ctx, u, 1, string(src))
	syms, _ := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})
	if list := syms.(protocol.DocumentSymbolSlice); len(list) < 5 {
		t.Fatalf("document symbols of std/array: %d", len(list))
	}
	again, _ := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})
	if len(again.(protocol.DocumentSymbolSlice)) != len(syms.(protocol.DocumentSymbolSlice)) {
		t.Fatal("the cached outline differs from the first")
	}
	ws, _ := s.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: "Array"})
	if len(ws.(protocol.SymbolInformationSlice)) != 0 {
		t.Fatalf("std symbols are not part of the workspace: %+v", ws)
	}
}
