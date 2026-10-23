package lsp_test

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

const typeUseSrc = "type Box = {\n    pub v Int\n}\n\nfn open(b: Box) -> Int {\n    b.v\n}\n\nlet made: Box = Box { v: 1 }\nlet n = open(made)\n"

func TestReferencesDoNotRepeatTypeAnnotations(t *testing.T) {
	s, ctx, doc, _ := openModule(t, typeUseSrc)
	refs, err := s.References(ctx, &protocol.ReferenceParams{TextDocumentPositionParams: at(doc, 0, 6), Context: protocol.ReferenceContext{IncludeDeclaration: true}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[protocol.Range]int{}
	for _, r := range refs {
		seen[r.Range]++
	}
	for r, n := range seen {
		if n > 1 {
			t.Errorf("range %+v returned %d times: %+v", r, n, refs)
		}
	}
	if len(seen) != 4 {
		t.Errorf("want the declaration plus 3 uses, got %d ranges: %+v", len(seen), refs)
	}
}

func TestDocumentHighlightDoesNotRepeatTypeAnnotations(t *testing.T) {
	s, ctx, doc, _ := openModule(t, typeUseSrc)
	hl, err := s.DocumentHighlight(ctx, &protocol.DocumentHighlightParams{TextDocumentPositionParams: at(doc, 0, 6)})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[protocol.Range]int{}
	for _, h := range hl {
		seen[h.Range]++
	}
	for r, n := range seen {
		if n > 1 {
			t.Errorf("range %+v highlighted %d times: %+v", r, n, hl)
		}
	}
	if len(seen) != 4 {
		t.Errorf("want 4 highlights, got %d: %+v", len(seen), hl)
	}
}

func TestPackageDotCompletionListsOnlyThePackage(t *testing.T) {
	src := "import json from std/json\n\nlet text = json.\n"
	s, ctx, doc, _ := openModule(t, src)
	items := completionLabels(t, s, ctx, doc, 2, 16)
	s2, ctx2, doc2, _ := openModule(t, "import os from std/os\n\nlet text = os.\n")
	if _, ok := items["encode"]; !ok {
		t.Errorf("json.encode missing: %v", labelsOf(items))
	}
	if _, ok := items["print"]; ok {
		t.Errorf("a name that is not a member of the package was offered: %v", labelsOf(items))
	}
	for _, name := range []string{"rt_sys_write", "rt_sys_exit", "rt_sys_close"} {
		if _, ok := completionLabels(t, s2, ctx2, doc2, 2, 14)[name]; ok {
			t.Errorf("runtime override point %s offered after `os.`", name)
		}
	}
	if len(items) > 80 {
		t.Errorf("%d items after `json.`, expected only the package's members", len(items))
	}
}

func TestDocumentSymbolRangeIncludesClosingBrace(t *testing.T) {
	src := "fn twice(n: Int) -> Int {\n    n + n\n}\n\ntype Box = {\n    pub v Int\n}\n"
	s, ctx, doc, _ := openModule(t, src)
	res, err := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}})
	if err != nil {
		t.Fatal(err)
	}
	ends := map[string]protocol.Position{}
	for _, sym := range res.(protocol.DocumentSymbolSlice) {
		ends[sym.Name] = sym.Range.End
	}
	if got := ends["twice"]; got.Line != 2 || got.Character != 1 {
		t.Errorf("twice ends at %+v, want line 2 col 1", got)
	}
	if got := ends["Box"]; got.Line != 6 || got.Character != 1 {
		t.Errorf("Box ends at %+v, want line 6 col 1", got)
	}
}

func TestDocumentSymbolRangeSurvivesInterpolationTokens(t *testing.T) {
	src := "fn first(n: Int) -> Int {\n    print \"" + strings.Repeat("${n}", 120) + "\"\n    n\n}\n\nfn second(n: Int) -> Int {\n    n + n\n}\n"
	s, ctx, doc, _ := openModule(t, src)
	res, err := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}})
	if err != nil {
		t.Fatal(err)
	}
	for _, sym := range res.(protocol.DocumentSymbolSlice) {
		if sym.Name == "second" && (sym.Range.End.Line != 7 || sym.Range.End.Character != 1) {
			t.Errorf("second ends at %+v, want line 7 col 1", sym.Range.End)
		}
	}
}

const genericUseSrc = "type W[T] = {\n    pub v T\n}\n\nfn use(w: W[i64]) -> i64 {\n    let k: W[W[i64]] = W { v: w }\n    k.v.v\n}\n"

func TestGenericTypeUsesAreNotRepeatedAndRenameDoesNotOverlap(t *testing.T) {
	s, ctx, doc, _ := openModule(t, genericUseSrc)
	refs, err := s.References(ctx, &protocol.ReferenceParams{TextDocumentPositionParams: at(doc, 0, 5), Context: protocol.ReferenceContext{IncludeDeclaration: true}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[protocol.Range]int{}
	for _, r := range refs {
		seen[r.Range]++
	}
	for r, n := range seen {
		if n > 1 || r.End.Character-r.Start.Character != 1 {
			t.Errorf("range %+v returned %d times: %+v", r, n, refs)
		}
	}
	if len(seen) != 5 {
		t.Errorf("want the declaration plus 4 uses, got %d: %+v", len(seen), refs)
	}
	edit, err := s.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at(doc, 0, 5), NewName: "Q"})
	if err != nil {
		t.Fatal(err)
	}
	var edits []protocol.TextEdit
	for _, es := range edit.Changes {
		edits = append(edits, es...)
	}
	if len(edits) != 5 {
		t.Errorf("want 5 rename edits, got %d: %+v", len(edits), edits)
	}
	for i, a := range edits {
		for _, b := range edits[i+1:] {
			if a.Range.Start.Line == b.Range.Start.Line && a.Range.Start.Character < b.Range.End.Character && b.Range.Start.Character < a.Range.End.Character {
				t.Errorf("overlapping edits %+v and %+v", a, b)
			}
		}
	}
}
