package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Staging is a temporary directory under root a caller populates before
// calling Commit, so a failed install never leaves a partial version directory.
type Staging struct {
	Dir     string
	root    string
	version string
}

// Stage creates a fresh temporary directory under root for installing
// version into; the caller must call either Commit or Discard.
func Stage(root, version string) (*Staging, error) {
	if err := validateVersion(version); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create zig store: %w", err)
	}
	dir, err := os.MkdirTemp(root, "install-"+version+"-")
	if err != nil {
		return nil, fmt.Errorf("create zig store: %w", err)
	}
	return &Staging{Dir: dir, root: root, version: version}, nil
}

// Commit renames the populated staging directory into place as root's
// version directory, replacing any previous install of the same version.
func (s *Staging) Commit() error {
	if err := validateVersion(s.version); err != nil {
		return err
	}
	dest := VersionDir(s.root, s.version)
	old := dest + ".replaced"
	os.RemoveAll(old)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			return fmt.Errorf("replace existing %s: %w", dest, err)
		}
	}
	if err := os.Rename(s.Dir, dest); err != nil {
		os.Rename(old, dest)
		return fmt.Errorf("install into %s: %w", dest, err)
	}
	os.RemoveAll(old)
	return nil
}

// Discard removes the staging directory without installing it.
func (s *Staging) Discard() {
	os.RemoveAll(s.Dir)
}

// Remove deletes an installed version from root; it is not an error for
// version to already be absent.
func Remove(root, version string) error {
	if err := validateVersion(version); err != nil {
		return err
	}
	dir := VersionDir(root, version)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	tmp := dir + ".removing"
	os.RemoveAll(tmp)
	if err := os.Rename(dir, tmp); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	return os.RemoveAll(tmp)
}

// Installed reports whether version is present in root with a usable zig
// binary, and its path.
func Installed(root, version string) (path string, ok bool) {
	if validateVersion(version) != nil {
		return "", false
	}
	dir := VersionDir(root, version)
	bin := BinaryPath(dir)
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return "", false
	}
	return bin, true
}

// List enumerates root's installed versions by directory name, skipping
// in-progress staging and removal directories.
func List(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list zig store %s: %w", root, err)
	}
	var versions []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) == ".replaced" || filepath.Ext(name) == ".removing" || strings.HasPrefix(name, "install-") {
			continue
		}
		if _, ok := Installed(root, name); ok {
			versions = append(versions, name)
		}
	}
	return versions, nil
}
