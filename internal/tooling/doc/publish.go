package doc

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"kigumi/internal/base/errors"
	"kigumi/internal/toolchain/driver"
)

const manifestName = ".kigumi-doc-manifest.json"

type siteManifest struct {
	Files []string `json:"files"`
}

// Publish writes files under dir via a staging directory that validate checks
// before it goes live. A dir holding only a previous run's own output is
// replaced wholesale; a shared dir is updated path-by-path via its manifest.
func Publish(dir string, files map[string][]byte, validate func(stagingDir string) error) error {
	full := withManifest(files)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return errors.WrapErr(err, "create doc output parent")
	}
	staging, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".staging-*")
	if err != nil {
		return errors.WrapErr(err, "create doc staging directory")
	}
	defer os.RemoveAll(staging)
	for name, content := range full {
		p := filepath.Join(staging, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return errors.WrapErr(err, "create doc staging directory")
		}
		if err := driver.WriteFileAtomic(p, content, 0o644); err != nil {
			return errors.WrapErr(err, "write staged doc page")
		}
	}
	if validate != nil {
		if err := validate(staging); err != nil {
			return err
		}
	}
	owned, hasManifest := readManifest(dir)
	if isDedicated(dir, owned, hasManifest) {
		if err := os.Chmod(staging, outputMode(dir)); err != nil {
			return errors.WrapErr(err, "create doc staging directory")
		}
		return swapIn(dir, staging)
	}
	return overlay(dir, full, owned)
}

func withManifest(files map[string][]byte) map[string][]byte {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	b, _ := json.MarshalIndent(siteManifest{Files: names}, "", "  ")
	full := make(map[string][]byte, len(files)+1)
	for k, v := range files {
		full[k] = v
	}
	full[manifestName] = append(b, '\n')
	return full
}

func readManifest(dir string) ([]string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, false
	}
	var m siteManifest
	if json.Unmarshal(b, &m) != nil {
		return nil, false
	}
	return m.Files, true
}

// isDedicated reports whether dir can be replaced wholesale: it is absent,
// or every file in it is one a previous manifest recorded as ours.
func isDedicated(dir string, owned []string, hasManifest bool) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return true
	}
	if !info.IsDir() {
		return false
	}
	walkRoot := dir
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		walkRoot = real
	}
	ownedSet := map[string]bool{manifestName: true}
	for _, o := range owned {
		ownedSet[o] = true
	}
	foreign := false
	filepath.WalkDir(walkRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel := filepath.ToSlash(mustRel(walkRoot, p))
		if !hasManifest || !ownedSet[rel] {
			foreign = true
		}
		return nil
	})
	return !foreign
}

func outputMode(dir string) os.FileMode {
	if info, err := os.Stat(dir); err == nil {
		return info.Mode().Perm()
	}
	return 0o755
}

func swapIn(dir, staging string) error {
	aside, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".replaced-*")
	if err != nil {
		return errors.WrapErr(err, "move previous doc output aside")
	}
	defer os.RemoveAll(aside)
	old := filepath.Join(aside, "previous")
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, old); err != nil {
			return errors.WrapErr(err, "move previous doc output aside")
		}
	}
	if err := os.Rename(staging, dir); err != nil {
		os.Rename(old, dir)
		return errors.WrapErr(err, "publish doc site")
	}
	return nil
}

func overlay(dir string, full map[string][]byte, owned []string) error {
	for name, content := range full {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return errors.WrapErr(err, "create doc directory")
		}
		if err := driver.WriteFileAtomic(p, content, 0o644); err != nil {
			return errors.WrapErr(err, "write doc page")
		}
	}
	for _, name := range owned {
		if _, ok := full[name]; ok {
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.Remove(p)
		removeEmptyParents(filepath.Dir(p), dir)
	}
	return nil
}

// removeEmptyParents removes dir and its ancestors up to (not including)
// root as long as each is empty; os.Remove no-ops on a non-empty one.
func removeEmptyParents(dir, root string) {
	for dir != root && dir != "." && dir != string(filepath.Separator) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
