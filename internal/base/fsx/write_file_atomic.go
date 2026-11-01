// Package fsx holds file-system helpers that know nothing about Kigumi.
package fsx

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic publishes data at path via a same-directory temp file so a crash never leaves a partial file; an existing file's mode is preserved, perm applies only when path is new.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
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
