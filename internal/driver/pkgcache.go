package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// PackageCacheStats counts package-cache traffic, so a test can pin that a no-op load writes nothing and a private edit rewrites one package.
type PackageCacheStats struct {
	// ArtifactWrites counts artifact bodies that were new to the store; RecordWrites counts key records, which a changed key adds even when the body it points at already exists.
	ArtifactWrites int
	RecordWrites   int
	ArtifactHits   int
	Corrupt        int
	SettledHits    int
	SettledWrites  int
	Pruned         int
}

// packageCache stores API artifacts by digest, key-to-digest records and settled check results; a read that fails to verify is a miss, never an error.
type packageCache struct {
	dir    string
	stats  PackageCacheStats
	writes int
	limit  int64
}

func newPackageCache(dir string) *packageCache {
	if dir == "" {
		return nil
	}
	return &packageCache{dir: dir}
}

// DefaultPackageCache is where the CLI keeps the package cache, beside the fetched-module cache.
func DefaultPackageCache(o CompilerOptions) (string, error) {
	dir, err := o.cacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kigumi", "pkg"), nil
}

// PackageCacheStats reports the cache traffic of this module; zero when the cache is off.
func (m *Module) PackageCacheStats() PackageCacheStats {
	if m.pkgCache == nil {
		return PackageCacheStats{}
	}
	return m.pkgCache.stats
}

func (c *packageCache) path(kind, name string) string {
	return filepath.Join(c.dir, kind, name[:2], name+".json")
}

type sealedFile struct {
	Sum  string          `json:"sum"`
	Body json.RawMessage `json:"body"`
}

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// putSealed writes v under a checksum, so a torn or edited file reads back as a miss.
func (c *packageCache) putSealed(kind, name string, v any) bool {
	body, err := json.Marshal(v)
	if err != nil {
		return false
	}
	b, err := json.Marshal(sealedFile{Sum: sumHex(body), Body: body})
	if err != nil || writeAtomic(c.path(kind, name), b) != nil {
		return false
	}
	c.noteWrite()
	return true
}

func (c *packageCache) getSealed(kind, name string, v any) bool {
	b, err := os.ReadFile(c.path(kind, name))
	if err != nil {
		return false
	}
	var f sealedFile
	if json.Unmarshal(b, &f) != nil || f.Sum != sumHex(f.Body) || json.Unmarshal(f.Body, v) != nil {
		c.stats.Corrupt++
		return false
	}
	return true
}

type keyRecord struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

func (c *packageCache) lookup(key string) (keyRecord, bool) {
	var rec keyRecord
	if !c.getSealed("pkg", key, &rec) || !c.hasAPI(rec.Digest) {
		return keyRecord{}, false
	}
	return rec, true
}

// putAPI stores an artifact under the hash of its own bytes and returns that digest; fresh is false when an intact copy was already there.
func (c *packageCache) putAPI(body []byte) (digest string, fresh, ok bool) {
	digest = sumHex(body)
	if c.hasAPI(digest) {
		return digest, false, true
	}
	if writeAtomic(c.path("api", digest), body) != nil {
		return digest, true, false
	}
	c.noteWrite()
	return digest, true, true
}

// hasAPI verifies the artifact against its name, since a record is only as good as the bytes it points at.
func (c *packageCache) hasAPI(digest string) bool {
	if len(digest) < 3 {
		return false
	}
	b, err := os.ReadFile(c.path("api", digest))
	if err != nil {
		return false
	}
	if sumHex(b) != digest {
		c.stats.Corrupt++
		return false
	}
	return true
}
