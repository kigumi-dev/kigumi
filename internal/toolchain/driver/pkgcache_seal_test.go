package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/module"
)

// TestPackageCacheSealsRecordsToThisAccount: records are sealed with the cache's own key, so a record whose checksum someone computed
// without it — the old unkeyed self-hash is the forged case — reads back as a miss instead of becoming a diagnostic `check` prints.
func TestPackageCacheSealsRecordsToThisAccount(t *testing.T) {
	t.Parallel()
	c := newPackageCache(filepath.Join(t.TempDir(), "pkg"))
	rec := keyRecord{Path: "p", Digest: "0123456789abcdef", Clean: true}
	if !c.putSealed("pkg", "abcdef", rec) {
		t.Fatal("a cache write must succeed on a fresh directory")
	}
	dir, err := os.Stat(c.dir)
	if err != nil || dir.Mode().Perm() != 0o700 {
		t.Fatalf("the cache directory must be owner-only, got %v (%v)", modeOf(dir), err)
	}
	file, err := os.Stat(c.path("pkg", "abcdef"))
	if err != nil || file.Mode().Perm() != 0o600 {
		t.Fatalf("a record must be owner-only, got %v (%v)", modeOf(file), err)
	}
	key, err := os.Stat(filepath.Join(c.dir, cacheKeyFile))
	if err != nil || key.Mode().Perm() != 0o600 {
		t.Fatalf("the cache key must be owner-only, got %v (%v)", modeOf(key), err)
	}
	var got keyRecord
	if !c.getSealed("pkg", "abcdef", &got) || got.Path != rec.Path || got.Digest != rec.Digest || got.Clean != rec.Clean {
		t.Fatalf("an unedited record must verify, got %+v", got)
	}

	body, err := json.Marshal(keyRecord{Path: "p", Digest: "0123456789abcdef", Clean: true})
	if err != nil {
		t.Fatal(err)
	}
	forged, err := json.Marshal(sealedFile{Sum: sumHex(body), Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.path("pkg", "abcdef"), forged, 0o600); err != nil {
		t.Fatal(err)
	}
	corrupt := c.stats.Corrupt
	if c.getSealed("pkg", "abcdef", &got) {
		t.Fatal("a record sealed with its own hash must be ignored")
	}
	if c.stats.Corrupt != corrupt+1 {
		t.Fatalf("a foreign record counts as corrupt, stats %+v", c.stats)
	}
}

// TestPackageCacheTightensSharedDirectory: a cache directory left group- or world-readable is made owner-only before anything is read
// from or written to it.
func TestPackageCacheTightensSharedDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "pkg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	c := newPackageCache(dir)
	if !c.putSealed("pkg", "abcdef", keyRecord{Path: "p"}) {
		t.Fatal("a directory this account owns is usable once tightened")
	}
	fi, err := os.Stat(dir)
	if err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("the directory must be tightened to 0700, got %v (%v)", modeOf(fi), err)
	}
}

// TestPackageCacheWriteWalksNothingUnderLimit: a write keeps the store's size in its own account, so unrelated files in the cache
// directory — someone else's junk, or a much older cache — cost nothing: the walk that knows their size only runs over the limit.
func TestPackageCacheWriteWalksNothingUnderLimit(t *testing.T) {
	// The walk counter is a package variable, so this test must not run beside another one using the cache.
	dir := filepath.Join(t.TempDir(), "pkg")
	shard := filepath.Join(dir, "pkg", "zz")
	if err := os.MkdirAll(shard, 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("unrelated")
	for i := range 2000 {
		if err := os.WriteFile(filepath.Join(shard, fmt.Sprintf("f%05d.json", i)), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	walks := 0
	pruneCache = func(dir string, kinds []string, limit int64) (int, int64) {
		walks++
		return 0, 0
	}
	defer func() { pruneCache = module.PruneCache }()

	c := newPackageCache(dir)
	if !c.putSealed("pkg", "abcdef", keyRecord{Path: "p"}) {
		t.Fatal("write failed")
	}
	if walks != 0 {
		t.Fatalf("a write under the limit must not walk the store, walked %d times", walks)
	}
	if c.bytes == 0 || c.bytes != c.loadAccount() {
		t.Fatalf("the write must be accounted (bytes=%d account=%d)", c.bytes, c.loadAccount())
	}

	// Past the limit the walk runs, and its total becomes the new account.
	c.limit = 1
	if !c.putSealed("pkg", "123456", keyRecord{Path: "p"}) {
		t.Fatal("write failed")
	}
	if walks != 1 {
		t.Fatalf("a write over the limit must walk once, walked %d times", walks)
	}
	if c.bytes != 0 || c.loadAccount() != 0 {
		t.Fatalf("the walk's total must replace the account, got %d/%d", c.bytes, c.loadAccount())
	}
}

func modeOf(fi os.FileInfo) any {
	if fi == nil {
		return nil
	}
	return fi.Mode().Perm()
}
