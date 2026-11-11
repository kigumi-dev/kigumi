package lsp

import (
	"path/filepath"
	"slices"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/token"
)

// exampleSnapshot analyzes examples/data_tools (several packages, an interface and its implementer, std imports).
func exampleSnapshot(t *testing.T) (*Server, *snapshot) {
	t.Helper()
	root, _ := filepath.Abs("../../../examples/data_tools")
	std, _ := filepath.Abs("../../../std")
	s := newServer(std).WithDebounce(0)
	s.roots = []string{root}
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.refresh(t.Context(), root, "", false)
	if snap.res == nil || snap.res.HasErrors() {
		t.Fatalf("examples/data_tools must analyze cleanly: %v", snap.err)
	}
	return s, snap
}

func TestPositionIndexMatchesTheScan(t *testing.T) {
	_, snap := exampleSnapshot(t)
	checked := 0
	for _, p := range snap.mod.Order {
		if p != "data_tools/report" && p != "data_tools/parse" && p != "std/array" {
			continue
		}
		for _, tree := range snap.mod.Packages[p].Files {
			step := 1 + len(tree.File.Src)/1500
			for pos := 0; pos <= len(tree.File.Src); pos += step {
				got, want := snap.posIndexOf(tree).nodeAt(token.Pos(pos)), scanNodeAt(tree, token.Pos(pos))
				if got != want {
					t.Fatalf("%s offset %d: index %d, scan %d", tree.File.Name, pos, got, want)
				}
				if got, want := snap.semIndex().declaredAt(tree, token.Pos(pos)), scanDeclaredAt(snap.res, tree, token.Pos(pos)); got != want {
					t.Fatalf("%s offset %d: declared index %d, scan %d", tree.File.Name, pos, got, want)
				}
				checked++
			}
		}
	}
	if checked < 500 {
		t.Fatalf("only %d offsets compared", checked)
	}
}

func TestUseAndConformanceIndexesMatchTheScans(t *testing.T) {
	_, snap := exampleSnapshot(t)
	res := snap.res
	uses, impls := 0, 0
	for i := range res.Entities {
		id := sem.EntityID(i)
		if res.Entity(id).File == 0 {
			continue
		}
		got, want := snap.semIndex().usesOf([]sem.EntityID{id}), scanUses(res, map[sem.EntityID]bool{id: true})
		if !slices.Equal(got, want) {
			t.Fatalf("uses of %s: index %v, scan %v", res.Entity(id).Name, got, want)
		}
		uses += len(got)
		gi, wi := snap.implementations(id), scanImplementations(res, id)
		if !slices.Equal(gi, wi) {
			t.Fatalf("implementations of %s: index %v, scan %v", res.Entity(id).Name, gi, wi)
		}
		impls += len(gi)
	}
	if uses < 100 || impls < 3 {
		t.Fatalf("the example is too thin to prove anything: %d uses, %d conformances", uses, impls)
	}
}

func TestSymbolTablesAreRebuiltPerChangedPackage(t *testing.T) {
	root := writeModule(t, map[string]string{
		"mod.kg":          "Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n",
		"alpha/alpha.kg":  "pub fn alphaOne() -> Int {\n    1\n}\n",
		"beta/beta.kg":    "pub fn betaOne() -> Int {\n    2\n}\n",
		"gamma/gamma.kg":  "pub fn gammaOne() -> Int {\n    3\n}\n",
		"cmd/app/main.kg": "import {alphaOne} from app/alpha\nprint \"${alphaOne()}\"\n",
	})
	s, ctx, _ := startServer(t, root, 10_000_000)
	betaFile := filepath.Join(root, "beta", "beta.kg")
	query := func() int {
		syms, err := s.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: "one"})
		if err != nil {
			t.Fatal(err)
		}
		return len(syms.(protocol.SymbolInformationSlice))
	}
	if got := query(); got != 3 {
		t.Fatalf("workspace symbols for `one`: %d", got)
	}
	built := s.Counters().SymbolPackages
	if got := query(); got != 3 || s.Counters().SymbolPackages != built {
		t.Fatalf("a repeated query must reuse the tables: %d symbols, %d rebuilt", got, s.Counters().SymbolPackages-built)
	}
	s.WithDebounce(0)
	openDoc(s, ctx, uri.File(betaFile), 1, "pub fn betaOne() -> Int {\n    2\n}\n\npub fn betaOneMore() -> Int {\n    3\n}\n")
	if got := query(); got != 4 {
		t.Fatalf("symbols after editing beta: %d", got)
	}
	if rebuilt := s.Counters().SymbolPackages - built; rebuilt != 1 {
		t.Fatalf("editing one package should rebuild one symbol table, rebuilt %d", rebuilt)
	}
}
