package arch_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/devkit/fixture"
)

// TestCorpusLoopsConsultTheShardSelector keeps heavy fixture corpora divisible: a test file that
// globs one of the shared fixture directories must slice it with the fixture shard selector, so the
// CI matrix stays balanced when the corpus grows. Files that regenerate a whole-corpus artifact on
// purpose are exempt.
func TestCorpusLoopsConsultTheShardSelector(t *testing.T) {
	corpusPath := regexp.MustCompile(`testdata/(run|sem|mir|parser|fmt|oom)/`)
	wholeCorpus := map[string]bool{
		"corpus_shard_test.go":  true, // this lint names the directories in its pattern
		"manifest_test.go":      true, // regenerates and diffs the manifest for the whole corpus
		"rc_call_count_test.go": true, // counts calls across the whole corpus; pinned to CI shard 0
	}
	err := filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if wholeCorpus[d.Name()] {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(src), "filepath.Glob") || !corpusPath.Match(src) {
			return nil
		}
		if !strings.Contains(string(src), ".Shards(") {
			t.Errorf("%s globs a shared fixture corpus without slicing it through the fixture shard selector", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Every corpus must split within one entry across the shards: a per-id hash put
// nearly twice as many mir or parser fixtures in one shard as in another.
func TestCorpusShardsBalance(t *testing.T) {
	root := "../.."
	for _, kind := range []string{"run", "sem", "mir", "parser", "fmt", "oom"} {
		pattern := filepath.Join(root, "testdata", kind, "*.txtar")
		if kind == "oom" {
			pattern = filepath.Join(root, "testdata", kind, "*.kg")
		}
		files, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatalf("no %s fixtures", kind)
		}
		counts := make([]int, 4)
		total := 0
		for shard := range 4 {
			t.Setenv("KIGUMI_TEST_SHARD", strconv.Itoa(shard)+"/4")
			counts[shard] = len(fixture.Shards(files))
			total += counts[shard]
		}
		if total != len(files) {
			t.Errorf("%s: shards cover %d of %d fixtures", kind, total, len(files))
		}
		least, most := slices.Min(counts), slices.Max(counts)
		if most-least > 1 {
			t.Errorf("%s: shard counts %v are not balanced", kind, counts)
		}
	}
}
