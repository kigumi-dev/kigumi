package driver

import (
	"os"
	"path/filepath"
)

// DeriveCacheDir is where `--plan` caches generated derive source, separate from CacheDir's fetched-module tree.
func DeriveCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kigumi", "derive"), nil
}
