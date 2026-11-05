package arch_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
		if !strings.Contains(string(src), ".Shard(") {
			t.Errorf("%s globs a shared fixture corpus without slicing it through the fixture shard selector", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
