package driver

import (
	"path/filepath"
)

// DeriveCacheDir is where `--plan` caches generated derive source, separate from CacheDir's fetched-module tree.
func DeriveCacheDir(o CompilerOptions) (string, error) {
	dir, err := o.cacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kigumi", "derive"), nil
}
