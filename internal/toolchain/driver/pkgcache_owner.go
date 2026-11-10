package driver

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// ownerOnlyDir makes dir exist, owner-only and this account's: created 0700, tightened when it was left wider, and refused when another
// account owns it or the path is not a real directory. A false return turns the package cache off rather than read records a stranger
// could have written.
func ownerOnlyDir(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	if fi.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return false
		}
	}
	return ownedBySelf(dir)
}

// cacheKey is the secret every record under dir is sealed with: created 0600 on first use and shared by this account's processes, so a
// record whose checksum was computed for another key reads back as a miss.
func cacheKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, cacheKeyFile)
	if key, err := os.ReadFile(path); err == nil {
		if len(key) != cacheKeyLen {
			return nil, fmt.Errorf("cache key %s is %d bytes, want %d", path, len(key), cacheKeyLen)
		}
		return key, nil
	}
	key := make([]byte, cacheKeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		// Another process of this account created the key first; use theirs.
		if b, rerr := os.ReadFile(path); rerr == nil && len(b) == cacheKeyLen {
			return b, nil
		}
		return nil, err
	}
	_, werr := f.Write(key)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(path)
		return nil, werr
	}
	return key, nil
}
