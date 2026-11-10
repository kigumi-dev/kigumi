package driver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
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
	dir   string
	stats PackageCacheStats
	limit int64
	// key seals every record, so a file this account's toolchain did not write is a miss and not a diagnostic a later check prints.
	key []byte
	// trusted marks dir prepared: this account owns it, it is owner-only, and key is loaded.
	trusted bool
	// bytes is the store's accounted size, kept in accountFile.
	bytes int64
}

func newPackageCache(dir string) *packageCache {
	if dir == "" {
		return nil
	}
	return &packageCache{dir: dir}
}

// ready prepares the cache directory on first use: created 0700, tightened if it was left wider, refused when another account owns it,
// and sealed with the per-cache key. A false return turns the cache off for this run, which only costs a recomputation.
func (c *packageCache) ready() bool {
	if c.trusted {
		return true
	}
	if !ownerOnlyDir(c.dir) {
		return false
	}
	key, err := cacheKey(c.dir)
	if err != nil {
		return false
	}
	c.key, c.bytes, c.trusted = key, c.loadAccount(), true
	return true
}

// DefaultPackageCache is where the CLI keeps the package cache, beside the fetched-module cache.
func DefaultPackageCache(o buildconfig.CompilerOptions) (string, error) {
	dir, err := o.CacheRoot()
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

// seal is the keyed checksum a record is stored under: a torn file fails it, and so does a whole record another account wrote, since
// the key never leaves this account's owner-only cache directory.
func (c *packageCache) seal(body []byte) string {
	m := hmac.New(sha256.New, c.key)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// putSealed writes v under the key's checksum, so a torn, edited or foreign file reads back as a miss.
func (c *packageCache) putSealed(kind, name string, v any) bool {
	if !c.ready() {
		return false
	}
	body, err := json.Marshal(v)
	if err != nil {
		return false
	}
	b, err := json.Marshal(sealedFile{Sum: c.seal(body), Body: body})
	if err != nil || writeAtomic(c.path(kind, name), b) != nil {
		return false
	}
	c.noteWrite(len(b))
	return true
}

func (c *packageCache) getSealed(kind, name string, v any) bool {
	if !c.ready() {
		return false
	}
	b, err := os.ReadFile(c.path(kind, name))
	if err != nil {
		return false
	}
	var f sealedFile
	if json.Unmarshal(b, &f) != nil || !hmac.Equal([]byte(f.Sum), []byte(c.seal(f.Body))) || json.Unmarshal(f.Body, v) != nil {
		c.stats.Corrupt++
		return false
	}
	return true
}

type keyRecord struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	// Clean marks a package that checked with no diagnostic, so a later run may skip it while its key holds.
	Clean bool `json:"clean,omitempty"`
	// CSymbols lists the C link names the package declares: E2218 is a rule about the whole module, so a replay has to know which names a
	// package it skips would bring into the check.
	CSymbols []string `json:"cSymbols,omitempty"`
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
	if !c.ready() || writeAtomic(c.path("api", digest), body) != nil {
		return digest, true, false
	}
	c.noteWrite(len(body))
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

const (
	cacheLimitBytes = 256 << 20
	// accountFile names the small record of how many bytes the store holds, so a write never has to walk it to find out.
	accountFile = "account.json"
	// cacheKeyFile holds the per-cache secret every record is sealed with.
	cacheKeyFile = "key"
	cacheKeyLen  = 32
)

var cacheKinds = []string{"pkg", "api", "check", "unit"}

// pruneCache is module.PruneCache through a variable, so a test can pin that a write under the limit walks nothing.
var pruneCache = module.PruneCache

// noteWrite accounts the n bytes just stored and prunes once the store passes its limit; the accounted size only grows by what this
// cache writes, so a directory holding someone else's files is never walked on the strength of its entry count.
func (c *packageCache) noteWrite(n int) {
	c.bytes += int64(n)
	c.saveAccount()
	limit := c.limit
	if limit == 0 {
		limit = cacheLimitBytes
	}
	if c.bytes > limit {
		c.prune()
	}
}

// prune drops the oldest files until the store is back under three quarters of its limit and takes the walk's own total as the new
// account, which is the only time the whole store is visited.
func (c *packageCache) prune() {
	if !c.ready() {
		return
	}
	limit := c.limit
	if limit == 0 {
		limit = cacheLimitBytes
	}
	pruned, remaining := pruneCache(c.dir, cacheKinds, limit)
	c.stats.Pruned += pruned
	c.bytes = remaining
	c.saveAccount()
}

// loadAccount reads the accounted size; a store with no account is one this version has not written to yet.
func (c *packageCache) loadAccount() int64 {
	var acc struct {
		Bytes int64 `json:"bytes"`
	}
	b, err := os.ReadFile(filepath.Join(c.dir, accountFile))
	if err != nil || json.Unmarshal(b, &acc) != nil || acc.Bytes < 0 {
		return 0
	}
	return acc.Bytes
}

func (c *packageCache) saveAccount() {
	b, err := json.Marshal(struct {
		Bytes int64 `json:"bytes"`
	}{c.bytes})
	if err != nil {
		return
	}
	_ = writeAtomic(filepath.Join(c.dir, accountFile), b)
}
