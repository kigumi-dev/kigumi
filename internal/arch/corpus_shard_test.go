package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/devkit/fixture"
)

// corpusDirLiteral matches a source string that names one of the shared fixture
// corpora directly. Corpus tests must go through fixture.Corpus / fixture.Find,
// so a group move changes paths in one place only.
var corpusDirLiteral = regexp.MustCompile(`testdata/(run|sem|mir|parser|fmt|oom)/`)

// TestCorpusLoopsUseFixtureFiles fails when Go source outside the fixture
// package builds a corpus path itself. The fixture package owns the
// testdata/<kind> layout; manifest_test.go may regenerate the whole manifest.
func TestCorpusLoopsUseFixtureFiles(t *testing.T) {
	root := "../.."
	skipDir := map[string]bool{".git": true, "testdata": true, "node_modules": true, "docs": true, "docs-new": true, ".work": true}
	exempt := map[string]bool{
		"corpus_shard_test.go": true, // this lint names the corpus directories in its pattern
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
		if !strings.HasSuffix(path, ".go") || exempt[d.Name()] {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if corpusDirLiteral.MatchString(s) {
				t.Errorf("%s: names a corpus directory directly (%q); list it with fixture.Corpus or locate one fixture with fixture.Find", path, s)
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
