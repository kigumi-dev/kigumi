package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/isr"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/policy"
)

// loadDeps loads the modules minimal version selection picks. enforceLock
// is false only for a script without <script>.lock.kg: its lock is opt-in.
func (m *Module) loadDeps(opts LoadOptions, lock module.Lock, enforceLock bool) error {
	for _, r := range m.Manifest.Requires {
		if r.Name == m.Name {
			return fmt.Errorf("dependency %s overlaps this module's name %s", r.Name, m.Name)
		}
	}
	sel, err := module.SelectVersionsResolved(m.compiler, m.Manifest, m.local, module.LockResolver(lock))
	if err != nil {
		return err
	}
	m.Selected = sel.Requires
	// The root's own packages carry Module=="" too; snapshotting them here keeps a dependency name that prefixes the root's own from claiming them.
	rootOwned := make(map[string]bool, len(m.index.pkgs))
	for p := range m.index.pkgs {
		rootOwned[p] = true
	}
	for _, key := range sel.Names() {
		r := sel.Requires[key]
		depDir, _, err := module.DepDir(m.compiler, m.local, r, true)
		if err != nil {
			return err
		}
		if _, replaced := m.local.Replacement(r); !replaced {
			if err := module.CheckCacheTree(depDir); err != nil {
				return err
			}
			entry, locked := lock.Entries[key]
			if locked && (entry.Alias != r.Alias || (entry.Name != "" && entry.Name != r.Name)) {
				// A lock written for another alias or module cannot verify this
				// Require even when it happens to share the map key.
				locked = false
			}
			if vendored, ok := m.local.Vendored[r.Name]; ok && m.local.Vendor != "" {
				// vendor/ is part of the checkout: compare it with its own
				// manifest, and still require the lock to name this version.
				if !locked || entry.Version != r.Version {
					if enforceLock {
						return fmt.Errorf("%s %s is not in %s; run `kigumi get` to lock it", module.DescribeRequire(r), r.Version, module.LockName)
					}
				} else if want := module.RepoURLKey(r); entry.URL != "" && want != "" && entry.URL != want {
					return fmt.Errorf("%s %s is fetched from %s but %s locked %s; run `kigumi get` to accept the new source", module.DescribeRequire(r), r.Version, want, module.LockName, entry.URL)
				}
				hash, err := module.ContentTreeHash(depDir)
				if err != nil {
					return err
				}
				if hash != vendored.Hash {
					return fmt.Errorf("%s %s in %s does not match %s; edit the dependency in its module, not in vendor/, or run `kigumi mod vendor` again", module.DescribeRequire(r), r.Version, depDir, filepath.Join(m.local.Vendor, module.VendorFileName))
				}
			} else if !locked || entry.Version != r.Version {
				if enforceLock {
					return fmt.Errorf("%s %s is not in %s; run `kigumi get` to lock it", module.DescribeRequire(r), r.Version, module.LockName)
				}
			} else {
				if want := module.RepoURLKey(r); entry.URL != "" && want != "" && entry.URL != want {
					return fmt.Errorf("%s %s is fetched from %s but %s locked %s; run `kigumi get` to accept the new source", module.DescribeRequire(r), r.Version, want, module.LockName, entry.URL)
				}
				hash, err := module.TreeHash(depDir)
				if err != nil {
					return err
				}
				if hash != entry.Hash {
					return fmt.Errorf("%s %s in %s does not match %s; the cache may be corrupted or tampered with, remove that directory and run `kigumi get` again", module.DescribeRequire(r), r.Version, depDir, module.LockName)
				}
			}
		}
		if prefix, covered := m.ownTreePrefixFor(depDir); covered {
			// A Replace may point inside the consumer's own root. Those files
			// are already indexed and checked as this module's own packages;
			// indexing the tree again under the dependency key would give one
			// file two nominal identities and report its diagnostics twice.
			// Imports spelled with the key resolve to the single copy instead.
			m.aliasDependencyPrefix(key, prefix)
			continue
		}
		depMod, _, err := module.ReadModFile(depDir)
		if err != nil {
			return err
		}
		if len(depMod.Interrupt.Handlers) > 0 {
			handlers := depMod.Interrupt.Handlers
			if r.Alias != "" {
				// A handler key names a package path; translate the dependency's
				// own real-name keys to the alias its packages carry.
				prefixes := map[string]string{r.Name: r.Alias}
				handlers = make(map[string][]string, len(depMod.Interrupt.Handlers))
				for k, allow := range depMod.Interrupt.Handlers {
					if i := strings.LastIndexByte(k, '.'); i > 0 {
						k = TranslateImportPrefixes(prefixes, k[:i]) + k[i:]
					}
					handlers[k] = allow
				}
			}
			m.depInterrupt = append(m.depInterrupt, sem.DepInterrupt{Module: key, Config: isr.Config{Handlers: handlers}})
		}
		if err := m.addNative(depDir, depMod); err != nil {
			return err
		}
		if err := m.addHeaders(depDir, depMod); err != nil {
			return err
		}
		// A dependency's cmd/ holds its own scripts and tools: entries of that module, not packages of ours.
		depCmd := filepath.Join(depDir, "cmd")
		skip := func(d string) bool {
			return module.IgnoredDir(opts.Compiler, d, opts.StdRoot) || module.IsModuleRoot(d) || module.SameDir(opts.Compiler, d, depCmd)
		}
		// A name-prefix relation alone is not an overlap; only an
		// actual package-path collision is.
		if overlap := m.dependencyOverlap(depDir, key, skip); overlap != "" {
			return fmt.Errorf("dependency %s overlaps this module's name %s", module.DescribeRequire(r), m.Name)
		}
		if err := m.indexTree(depDir, key, false, false, skip); err != nil {
			return err
		}
		for path, ip := range m.index.pkgs {
			if ip.module == "" && !ip.std && !rootOwned[path] && module.OwnerOf(path, []string{key}) != "" {
				// The effective key, not the real name, is the package's
				// module identity: two aliases of one real module are
				// distinct modules for visibility and cycle checks.
				ip.module = key
				if r.Alias != "" {
					ip.importPrefixes = map[string]string{r.Name: r.Alias}
				}
			}
		}
	}
	return nil
}

// ownTreePrefixFor reports the package-path prefix the root's own walk gave
// dir, and whether that walk indexed a package at or below it. A Replace may
// point inside the consumer's root, where the tree is already part of the
// module.
func (m *Module) ownTreePrefixFor(dir string) (string, bool) {
	root, err := m.compiler.Abs(m.Root)
	if err != nil {
		return "", false
	}
	abs, err := m.compiler.Abs(dir)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	covered := false
	for _, ip := range m.index.pkgs {
		if ip.std || ip.module != "" {
			continue
		}
		pdir, err := m.compiler.Abs(ip.dir)
		if err != nil {
			continue
		}
		sub, err := filepath.Rel(abs, pdir)
		if err == nil && sub != ".." && !strings.HasPrefix(sub, ".."+string(filepath.Separator)) {
			covered = true
			break
		}
	}
	if !covered {
		return "", false
	}
	prefix := m.Name
	if rel != "." {
		if prefix == "" {
			prefix = filepath.ToSlash(rel)
		} else {
			prefix += "/" + filepath.ToSlash(rel)
		}
	}
	if prefix == "" {
		return "", false
	}
	return prefix, true
}

// aliasDependencyPrefix makes imports spelled with the dependency key key
// resolve to prefix, the path the root's own tree already uses for the same
// files, so the two spellings name one package. Only own packages learn the
// mapping: a dependency's own import of that spelling does not mean the
// consumer's package.
func (m *Module) aliasDependencyPrefix(key, prefix string) {
	if key == prefix || m.ownsPackagePrefix(key) {
		return
	}
	for _, ip := range m.index.pkgs {
		if ip.std || ip.module != "" {
			continue
		}
		if ip.importPrefixes == nil {
			ip.importPrefixes = map[string]string{}
		}
		ip.importPrefixes[key] = prefix
	}
	for _, p := range m.Packages {
		if p.Std || p.Module != "" {
			continue
		}
		if p.ImportPrefixes == nil {
			p.ImportPrefixes = map[string]string{}
		}
		p.ImportPrefixes[key] = prefix
	}
}

// ownsPackagePrefix reports whether the root's own packages already answer
// imports under prefix, which would make an alias redirect them.
func (m *Module) ownsPackagePrefix(prefix string) bool {
	if _, ok := m.index.pkgs[prefix]; ok {
		return true
	}
	lead := prefix + "/"
	for path := range m.index.pkgs {
		if strings.HasPrefix(path, lead) {
			return true
		}
	}
	return false
}

// addNative collects a module's C libraries/sources resolved against its directory; each path is re-confined since the manifest's lexical check can't see symlinks.
func (m *Module) addNative(dir string, mf module.ModFile) error {
	for _, l := range mf.Links {
		if l.Search != "" {
			l.Search = filepath.Join(dir, filepath.FromSlash(l.Search))
			if !withinSandbox([]string{dir}, l.Search) {
				return fmt.Errorf("%s: Link search path %q resolves outside the module", mf.Name, l.Search)
			}
			if _, err := os.Stat(l.Search); err != nil {
				m.Violations = append(m.Violations, policy.NativeLinkUnresolvedViolation(module.DisplayName(mf), l.Search))
			}
		}
		m.Links = append(m.Links, l)
	}
	for _, s := range mf.Sources {
		files, err := m.selectSources(filepath.Join(dir, filepath.FromSlash(s.Path)), s.Asm)
		if err != nil {
			return fmt.Errorf("%s: Source %q: %w", mf.Name, s.Path, err)
		}
		for _, f := range files {
			if !withinSandbox([]string{dir}, f) {
				rel, _ := filepath.Rel(dir, f)
				return fmt.Errorf("%s: Source %q resolves outside the module", mf.Name, filepath.ToSlash(rel))
			}
			m.Sources = append(m.Sources, f)
		}
	}
	return nil
}

func (m *Module) addExtraNative(opts LoadOptions) {
	m.Sources = append(m.Sources, opts.ExtraSources...)
	m.Links = append(m.Links, opts.ExtraLinks...)
}

// selectSources expands a Source candidate into its matching files.
// A directory that fails to list returns an error; an empty one does not.
func (m *Module) selectSources(path string, asm bool) ([]string, error) {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		if tagged, ok := m.platformFile(filepath.Base(path)); tagged && !ok {
			return nil, nil
		}
		return []string{path}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)
		if e.IsDir() || (asm && ext != ".s" && ext != ".S") || (!asm && ext != ".c") {
			continue
		}
		if tagged, ok := m.platformFile(name); tagged && !ok {
			continue
		}
		out = append(out, filepath.Join(path, name))
	}
	return out, nil
}

// ImportedModules lists the Requires that the module's own packages
// import, matching import paths to the longest effective Require key (the
// alias when one is given).
func (m *Module) ImportedModules(mf module.ModFile) map[string]bool {
	m.ensureParsed()
	names := make([]string, 0, len(mf.Requires))
	for _, r := range mf.Requires {
		names = append(names, r.Key())
	}
	used := map[string]bool{}
	for _, p := range m.Packages {
		if p.Std || p.Module != "" {
			continue
		}
		for _, t := range p.Files {
			for _, path := range importPaths(t) {
				if owner := module.OwnerOf(path, names); owner != "" {
					used[owner] = true
				}
			}
		}
	}
	return used
}
