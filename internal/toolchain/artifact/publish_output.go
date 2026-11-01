package artifact

import (
	"fmt"
	"os"
	"path/filepath"
)

// Publish stages build's output and only renames it over out once it's a non-empty regular file, so a failed in-place linker/archiver can't destroy the last good out.
func Publish(out string, build func(staged string) error) (bool, error) {
	dir := filepath.Dir(out)
	f, err := os.CreateTemp(dir, "."+filepath.Base(out)+".build-")
	if err != nil {
		return false, err
	}
	staged := f.Name()
	f.Close()
	// ar refuses an existing empty file as a fresh archive; every build
	// (cc/ld included) is fine with the name just not existing yet.
	os.Remove(staged)
	defer os.Remove(staged)
	if err := build(staged); err != nil {
		return false, err
	}
	fi, err := os.Lstat(staged)
	if err != nil {
		return false, fmt.Errorf("build reported success but wrote nothing at %s: %w", out, err)
	}
	if !fi.Mode().IsRegular() || fi.Size() == 0 {
		return false, fmt.Errorf("build reported success but left no usable output at %s", out)
	}
	if err := os.Rename(staged, out); err != nil {
		return false, err
	}
	return true, nil
}
