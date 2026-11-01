package module

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// OriginFileName marks a complete cache entry: `kigumi get` writes the
// commit it cloned into it last, so a directory without one is unfinished.
const OriginFileName = ".kigumi-origin"

// readFileMaybe reads path; ok is false only when the file does not exist.
// Any other failure (permissions, I/O) is an error, never "missing".
func readFileMaybe(path string) (data []byte, ok bool, err error) {
	data, err = os.ReadFile(path)
	switch {
	case err == nil:
		return data, true, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	}
	return nil, false, err
}

// CheckCacheTree refuses a fetched module holding anything but regular files and directories, since a symlink could lead the compiler outside the cache.
func CheckCacheTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if t := d.Type(); t&fs.ModeSymlink != 0 || t&(fs.ModeDevice|fs.ModeNamedPipe|fs.ModeSocket|fs.ModeCharDevice) != 0 {
			rel, _ := filepath.Rel(dir, path)
			return fmt.Errorf("fetched module %s holds %s, which is not a regular file; refusing to use the cache entry", dir, filepath.ToSlash(rel))
		}
		return nil
	})
}

// CacheReady reports a complete cache entry: the origin marker and the
// manifest are both present.
func CacheReady(dir string) bool {
	_, okOrigin, _ := readFileMaybe(filepath.Join(dir, OriginFileName))
	_, okManifest, _ := readFileMaybe(filepath.Join(dir, ManifestName))
	return okOrigin && okManifest
}
