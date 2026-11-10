package module

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeCacheFiles puts n files of size bytes under dir/kind/shard, the layout the package cache uses.
func writeCacheFiles(t *testing.T, dir, kind string, n, size int) {
	t.Helper()
	shard := filepath.Join(dir, kind, "aa")
	if err := os.MkdirAll(shard, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := range n {
		body := fmt.Sprintf("%0*d", size, i)
		if err := os.WriteFile(filepath.Join(shard, fmt.Sprintf("f%04d.json", i)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPruneCacheDropsOldestOverLimit: over the limit the oldest files go first, down to three quarters, and the remaining total is
// reported so the caller can account for the store without walking it again.
func TestPruneCacheDropsOldestOverLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCacheFiles(t, dir, "pkg", 40, 200)
	pruned, remaining := PruneCache(dir, []string{"pkg"}, 4000)
	if pruned == 0 {
		t.Fatal("an over-limit store must prune")
	}
	if remaining > 3000 || remaining >= 40*200 {
		t.Fatalf("pruning must report what is left down to three quarters, got %d", remaining)
	}
	var walked int64
	filepath.WalkDir(filepath.Join(dir, "pkg"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			st, _ := d.Info()
			walked += st.Size()
		}
		return nil
	})
	if walked != remaining {
		t.Errorf("reported %d bytes remaining, the directory holds %d", remaining, walked)
	}
}

// TestPruneCacheKeepsUnderLimitStoreWhole: a store under its limit keeps every file, and the walk reports its size.
func TestPruneCacheKeepsUnderLimitStoreWhole(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCacheFiles(t, dir, "pkg", 5, 100)
	pruned, remaining := PruneCache(dir, []string{"pkg"}, 1<<20)
	if pruned != 0 || remaining != 500 {
		t.Fatalf("pruned=%d remaining=%d, want 0 and 500", pruned, remaining)
	}
	if _, err := os.Stat(filepath.Join(dir, "pkg", "aa", "f0000.json")); err != nil {
		t.Errorf("an under-limit store must keep every file: %v", err)
	}
}
