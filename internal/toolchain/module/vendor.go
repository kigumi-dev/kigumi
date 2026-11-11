package module

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kigumi/internal/base/fsx"
	buildconfig "kigumi/internal/toolchain/config"
)

// Vendoring copies the modules the lock pins into `<root>/vendor/<module
// path>`, so a checkout builds without the module cache. `vendor/modules.txt`
// lists what is there, with the version, commit and content tree hash of each
// module; the same lock always produces the same bytes.

// VendorDirName is the repo-local directory holding vendored modules.
const VendorDirName = "vendor"

// VendorFileName is the manifest of a vendor directory.
const VendorFileName = "modules.txt"

// VendorEntry is one line of vendor/modules.txt.
type VendorEntry struct {
	Version, Commit, Hash string
}

// Vendor is a parsed vendor directory. Entries is empty for a directory
// without modules.txt, which is not a vendor tree this toolchain wrote.
type Vendor struct {
	Dir     string
	Entries map[string]VendorEntry
}

// VendorDir is the vendor directory of the module rooted at root.
func VendorDir(root string) string { return filepath.Join(root, VendorDirName) }

// ReadVendor reads root/vendor/modules.txt. ok is false when root has no
// vendored tree, or has a vendor directory without our manifest (a manually
// maintained tree resolved through mod.local.kg).
func ReadVendor(root string) (Vendor, bool, error) {
	dir := VendorDir(root)
	file := filepath.Join(dir, VendorFileName)
	src, ok, err := readFileMaybe(file)
	if err != nil {
		return Vendor{}, false, err
	}
	if !ok {
		return Vendor{}, false, nil
	}
	v := Vendor{Dir: dir, Entries: map[string]VendorEntry{}}
	for i, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 {
			return Vendor{}, true, fmt.Errorf("%s:%d: expected `name version commit hash`", file, i+1)
		}
		name, version, commit, hash := fields[0], fields[1], fields[2], fields[3]
		if err := ValidModuleName(name); err != nil {
			return Vendor{}, true, fmt.Errorf("%s:%d: %w", file, i+1, err)
		}
		if err := ValidVersion(version); err != nil {
			return Vendor{}, true, fmt.Errorf("%s:%d: %w", file, i+1, err)
		}
		if err := CheckVersionForName(name, version); err != nil {
			return Vendor{}, true, fmt.Errorf("%s:%d: %w", file, i+1, err)
		}
		if !strings.HasPrefix(hash, "h1:") {
			return Vendor{}, true, fmt.Errorf("%s:%d: %q is not an h1 tree hash", file, i+1, hash)
		}
		if _, dup := v.Entries[name]; dup {
			return Vendor{}, true, fmt.Errorf("%s:%d: %s is listed twice", file, i+1, name)
		}
		v.Entries[name] = VendorEntry{Version: version, Commit: commit, Hash: hash}
	}
	return v, true, nil
}

// VendoredModule is one module copied into vendor/.
type VendoredModule struct {
	Name, Version, Commit, Hash string
}

// VendorTree copies every module selection picks, at the version the lock
// pins, into root/vendor and writes root/vendor/modules.txt. It verifies each
// cache entry against the lock before copying and re-hashes the copy, so the
// recorded hashes describe exactly what is on disk.
//
// A module that mod.local.kg replaces is refused: the vendor tree must be the
// released dependency, not a developer's checkout.
func VendorTree(o buildconfig.CompilerOptions, root string) ([]VendoredModule, error) {
	mf, ok, err := ReadModFile(root)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s has no %s", root, ManifestName)
	}
	lock, err := ReadLock(root)
	if err != nil {
		return nil, err
	}
	local, err := ReadLocal(o, root)
	if err != nil {
		return nil, err
	}
	sel, err := SelectVersions(o, mf, local)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(VendorDir(root)); err == nil {
		if _, ours, err := ReadVendor(root); err != nil {
			return nil, err
		} else if !ours {
			return nil, fmt.Errorf("%s exists but has no %s; it was not written by `kigumi mod vendor`, so it will not be replaced", VendorDir(root), VendorFileName)
		}
	}
	var mods []VendoredModule
	for _, name := range sel.Names() {
		r := sel.Requires[name]
		if _, replaced := local.Replaces[name]; replaced {
			return nil, fmt.Errorf("%s is replaced by %s; a vendored tree holds the locked dependency, so commit the checkout or drop the Replace first", name, LocalName)
		}
		entry, locked := lock.Entries[name]
		if !locked || entry.Version != r.Version {
			return nil, fmt.Errorf("%s %s is not in %s at that version; run `kigumi get` first", name, r.Version, LockName)
		}
		dir, err := CacheDir(o, r)
		if err != nil {
			return nil, err
		}
		if !CacheReady(dir) {
			return nil, fmt.Errorf("%s %s is not fetched into %s; run `kigumi get` first", name, r.Version, dir)
		}
		if err := CheckCacheTree(dir); err != nil {
			return nil, err
		}
		hash, err := TreeHash(dir)
		if err != nil {
			return nil, err
		}
		if hash != entry.Hash {
			return nil, fmt.Errorf("%s %s in %s does not match %s; remove that directory and run `kigumi get` again", name, r.Version, dir, LockName)
		}
		mods = append(mods, VendoredModule{Name: name, Version: r.Version, Commit: entry.Commit, Hash: ""})
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Name < mods[j].Name })
	if len(mods) == 0 {
		return nil, removeVendor(root)
	}

	tmp, err := os.MkdirTemp(root, ".vendor-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	for i, m := range mods {
		cacheDir, err := CacheDir(o, Require{Name: m.Name, Version: m.Version})
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(tmp, filepath.FromSlash(m.Name))
		if err := copyModuleTree(cacheDir, dst); err != nil {
			return nil, fmt.Errorf("%s %s: %w", m.Name, m.Version, err)
		}
		hash, err := ContentTreeHash(dst)
		if err != nil {
			return nil, err
		}
		srcHash, err := ContentTreeHash(cacheDir)
		if err != nil {
			return nil, err
		}
		if hash != srcHash {
			return nil, fmt.Errorf("%s %s: the vendored copy %s does not reproduce the cached tree %s", m.Name, m.Version, hash, srcHash)
		}
		mods[i].Hash = hash
	}
	if err := writeVendorManifest(tmp, mods); err != nil {
		return nil, err
	}
	if err := replaceDir(tmp, VendorDir(root)); err != nil {
		return nil, err
	}
	return mods, nil
}

// writeVendorManifest writes the module list in a stable order; only paths,
// versions, commits and content hashes appear, so a rerun is byte-identical.
func writeVendorManifest(dir string, mods []VendoredModule) error {
	var sb strings.Builder
	sb.WriteString("// Generated by `kigumi mod vendor`; do not edit.\n")
	sb.WriteString("// name version commit hash\n")
	for _, m := range mods {
		fmt.Fprintf(&sb, "%s %s %s %s\n", m.Name, m.Version, m.Commit, m.Hash)
	}
	return fsx.WriteFileAtomic(filepath.Join(dir, VendorFileName), []byte(sb.String()), 0o644)
}

// copyModuleTree copies a cache entry into dst, leaving the origin marker
// behind: it records a cache's commit, not module content.
func copyModuleTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if rel == OriginFileName {
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s is a symlink; refusing to vendor it", filepath.ToSlash(rel))
		case !d.Type().IsRegular():
			return fmt.Errorf("%s is not a regular file; refusing to vendor it", filepath.ToSlash(rel))
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		perm := os.FileMode(0o644)
		if info.Mode()&0o111 != 0 {
			perm = 0o755
		}
		return os.WriteFile(target, data, perm)
	})
}

// replaceDir puts staging at dst, keeping the previous directory only until
// the rename succeeds so a failed vendor run leaves the old tree in place.
func replaceDir(staging, dst string) error {
	backup := staging + "-old"
	hadOld := false
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Rename(dst, backup); err != nil {
			return err
		}
		hadOld = true
	}
	if err := os.Rename(staging, dst); err != nil {
		if hadOld {
			os.Rename(backup, dst)
		}
		return err
	}
	if hadOld {
		os.RemoveAll(backup)
	}
	return nil
}

// removeVendor deletes the vendor tree this toolchain wrote, if there is one;
// a hand-maintained vendor directory is left alone.
func removeVendor(root string) error {
	if _, ok, err := ReadVendor(root); err != nil {
		return err
	} else if !ok {
		return nil
	}
	return os.RemoveAll(VendorDir(root))
}
