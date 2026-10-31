package driver

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

// writeFileAtomic publishes data at path via a same-directory temp file so a crash never leaves a partial file; an existing file's mode is preserved, perm applies only when path is new.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	// Lstat, not Stat: the rename below replaces the entry itself, so a
	// symlink there is refused rather than silently followed.
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink; refusing to replace it with a regular file", path)
		}
		perm = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// WriteFileAtomic is writeFileAtomic exported for callers outside this package (kigumi fmt -w), for the same crash-safety and mode preservation.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeFileAtomic(path, data, perm)
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
