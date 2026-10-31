package driver

import (
	"path/filepath"

	buildconfig "kigumi/internal/toolchain/config"
)

// DeriveCacheDir is where `--plan` caches generated derive source, separate from CacheDir's fetched-module tree.
func DeriveCacheDir(o buildconfig.CompilerOptions) (string, error) {
	dir, err := o.CacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kigumi", "derive"), nil
}
