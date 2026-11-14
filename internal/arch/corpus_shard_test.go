package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/devkit/fixture"
)

// corpusKinds is derived from the fixture package so this lint follows a kind
// that is renamed or added instead of keeping a second list in sync.
var corpusKinds = func() map[string]bool {
	kinds := make(map[string]bool, len(fixture.Kinds))
	for _, kind := range fixture.Kinds {
		kinds[kind] = true
	}
	return kinds
}()

// corpusPathViolation reports whether a string literal spells out where the
// shared fixtures live: a "testdata" path segment followed by a corpus kind, or
// a repo-rooted handle that ends at "testdata" and is therefore combined with a
// kind by its caller. Prose such as "fixtures belong in testdata/sem" has no
// bare "testdata" segment and does not match.
func corpusPathViolation(path string) bool {
	segs := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for i, seg := range segs {
		if seg != "testdata" {
			continue
		}
		if i+1 < len(segs) && corpusKinds[segs[i+1]] {
			return true
		}
		if i == len(segs)-1 && slices.Contains(segs[:i], "..") {
			return true
		}
	}
	return false
}

// isJoinCall reports whether fun is filepath.Join or path.Join.
func isJoinCall(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && (x.Name == "filepath" || x.Name == "path")
}

// TestCorpusSourcesUseFixtureAPI fails when Go source outside the fixture
// package builds a corpus path itself. The fixture package owns the
// testdata/<kind> layout and the group a fixture lives in, so a test must
// enumerate with fixture.Corpus/fixture.Files and locate one fixture with
// fixture.Find; a group move then changes paths in one place only.
func TestCorpusSourcesUseFixtureAPI(t *testing.T) {
	root := "../.."
	skipDir := map[string]bool{".git": true, "testdata": true, "node_modules": true, "docs": true, "docs-new": true, ".work": true}
	// allow lists the exact literals a file may still spell out under testdata.
	// Every entry is a non-corpus use: a whole-tree source sweep, not a lookup
	// of a fixture by kind or group.
	allow := map[string]map[string]bool{
		// Walks every shipped .kg and .txtar source under testdata to feed it
		// through the parser; it never looks a corpus entry up by kind.
		"internal/compiler/syntax/schema_validate_internal_test.go": {"../../../testdata": true},
		// Sweeps every shipped .kg file (the oom corpus stores .kg sources) to
		// check LSP symbol ranges; the recursive walk is the point.
		"internal/tooling/lsp/symbols_test.go": {"../../../testdata": true},
	}
	// This lint's allowlist necessarily spells an allowed path.
	skipFile := map[string]bool{"internal/arch/corpus_shard_test.go": true}
	report := func(rel, literal string) {
		t.Helper()
		if rel == "" || skipFile[rel] || allow[rel][literal] {
			return
		}
		t.Errorf("%s: names a corpus path directly (%q); enumerate with fixture.Corpus and locate one fixture with fixture.Find", rel, literal)
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir[d.Name()] {
				return filepath.SkipDir
			}
			if filepath.ToSlash(path) == filepath.ToSlash(filepath.Join(root, "internal", "devkit", "fixture")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BasicLit:
				if n.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(n.Value)
				if err != nil {
					return true
				}
				if corpusPathViolation(s) {
					report(rel, s)
				}
			case *ast.CallExpr:
				// A split join such as Join("../../..", "testdata", kind)
				// hides the corpus root across two literals; rejoin the
				// all-literal arguments before testing them.
				if !isJoinCall(n.Fun) {
					return true
				}
				parts := make([]string, 0, len(n.Args))
				for _, arg := range n.Args {
					lit, ok := arg.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					s, err := strconv.Unquote(lit.Value)
					if err != nil {
						return true
					}
					parts = append(parts, s)
				}
				if joined := strings.Join(parts, "/"); corpusPathViolation(joined) {
					for _, part := range parts {
						if allow[rel][part] {
							return true
						}
					}
					report(rel, joined)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCorpusLoopsConsultTheShardSelector fails when a test that enumerates a
// shared corpus does not slice it through the fixture shard selector.
// Whole-corpus checks are exempt because they must see every fixture at once.
func TestCorpusLoopsConsultTheShardSelector(t *testing.T) {
	root := "../.."
	wholeCorpus := map[string]bool{
		"manifest_test.go":                 true, // regenerates and diffs the manifest for the whole corpus
		"rc_call_count_test.go":            true, // counts calls across the whole corpus; pinned to CI shard 0
		"alloc_fault_try_coverage_test.go": true, // reads manifest metadata, runs no fixture
		"alloc_fault_try_family_test.go":   true, // sweeps the whole fault corpus in one CI job
		"corpus_shard_test.go":             true, // this lint enumerates the corpora itself
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "testdata" || d.Name() == "docs-new" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") || wholeCorpus[d.Name()] {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(src)
		if !strings.Contains(text, ".Corpus(") {
			return nil
		}
		if strings.Contains(text, ".ShardPaths(") || strings.Contains(text, ".Shards(") {
			return nil
		}
		t.Errorf("%s enumerates a shared corpus without slicing it through fixture.ShardPaths", path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Every corpus must split within one entry across the shards: a per-path hash
// would put nearly twice as many fixtures in one shard as in another once the
// group became part of the path.
func TestCorpusShardsBalance(t *testing.T) {
	for _, kind := range fixture.Kinds {
		paths, err := fixture.Corpus("../..", kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) == 0 {
			t.Fatalf("no %s fixtures", kind)
		}
		ids := make([]string, len(paths))
		for i, path := range paths {
			ids[i] = fixture.IDFromPath(path)
		}
		counts := make([]int, 4)
		total := 0
		for shard := range 4 {
			t.Setenv("KIGUMI_TEST_SHARD", strconv.Itoa(shard)+"/4")
			counts[shard] = len(fixture.Shards(ids))
			total += counts[shard]
		}
		if total != len(ids) {
			t.Errorf("%s: shards cover %d of %d fixtures", kind, total, len(ids))
		}
		least, most := slices.Min(counts), slices.Max(counts)
		if most-least > 1 {
			t.Errorf("%s: shard counts %v are not balanced", kind, counts)
		}
	}
}

// The four shards must partition each corpus exactly: every id in exactly one
// shard, and no shard inventing an id.
func TestCorpusShardCoverage(t *testing.T) {
	for _, kind := range fixture.Kinds {
		paths, err := fixture.Corpus("../..", kind)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{}
		for _, path := range paths {
			want[fixture.IDFromPath(path)] = true
		}
		seen := map[string]int{}
		for shard := range 4 {
			t.Setenv("KIGUMI_TEST_SHARD", strconv.Itoa(shard)+"/4")
			ids := make([]string, len(paths))
			for i, path := range paths {
				ids[i] = fixture.IDFromPath(path)
			}
			for _, id := range fixture.Shards(ids) {
				if !want[id] {
					t.Errorf("%s: shard %d selected unknown id %q", kind, shard, id)
				}
				seen[id]++
			}
		}
		for id := range want {
			if seen[id] != 1 {
				t.Errorf("%s: id %q ran in %d shards, want 1", kind, id, seen[id])
			}
		}
	}
}
