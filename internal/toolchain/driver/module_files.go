package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// indexPackage records the files of one directory as a package header, without reading them.
func (m *Module) indexPackage(dir, pkgPath, fileRel string, std, test bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	// build.kg is the build program only at the module root; a package may
	// still have a file of that name (std/build does).
	isProgram := func(name string) bool { return name == BuildFileName && sameDir(m.compiler, dir, m.Root) }
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".kg") && !IsManifest(e.Name()) && !isProgram(e.Name()) {
			names[e.Name()] = true
		}
	}
	for abs := range m.overlay {
		if filepath.Dir(abs) == filepath.Clean(dir) && strings.HasSuffix(abs, ".kg") && !IsManifest(filepath.Base(abs)) && !isProgram(filepath.Base(abs)) {
			names[filepath.Base(abs)] = true
		}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	if m.fileOrder != nil {
		m.fileOrder(sorted)
	}
	var files []*indexedFile
	for _, name := range sorted {
		if strings.HasSuffix(name, "_test.kg") && !test {
			continue
		}
		if tagged, ok := m.platformFile(name); tagged && !ok {
			continue
		}
		full := filepath.Join(dir, name)
		// Root-relative names keep golden output independent of the checkout path.
		display := name
		if fileRel != "" {
			display = fileRel + "/" + name
		}
		f := &indexedFile{name: name, display: display, abs: full}
		if src, ok := m.overlay[full]; ok {
			f.src, f.loaded, f.overlay, f.size = src, true, true, int64(len(src))
		} else {
			st, err := os.Stat(full)
			if err != nil {
				return err
			}
			f.size, f.mtime, f.ctime = st.Size(), st.ModTime().UnixNano(), changeTime(st)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return nil
	}
	// Files at the root of a manifest-less directory form package main:
	// the entry of a single-script module, which nothing needs to import.
	if pkgPath == "" {
		pkgPath = "main"
	}
	if _, dup := m.index.pkgs[pkgPath]; dup {
		return fmt.Errorf("package %q is defined by two directories", pkgPath)
	}
	m.index.pkgs[pkgPath] = &indexedPackage{path: pkgPath, dir: dir, std: std, files: files}
	m.index.order = append(m.index.order, pkgPath)
	m.idxStats.PackagesIndexed++
	return nil
}

// parsePackage parses an indexed package and publishes it in m.Packages.
func (m *Module) parsePackage(ip *indexedPackage) error {
	if ip.parsed {
		return nil
	}
	var trees []*syntax.Tree
	for _, f := range ip.files {
		src, err := m.source(f)
		if err != nil {
			return err
		}
		file := token.NewFile(f.display, src)
		m.fileStore().Add(file)
		trees = append(trees, syntax.Parse(file))
	}
	ip.parsed = true
	m.idxStats.PackagesParsed++
	m.idxStats.FilesParsed += len(trees)
	m.Packages[ip.path] = &Package{Path: ip.path, Dir: ip.dir, Files: trees, Std: ip.std, Module: ip.module}
	return nil
}

// loadPackage indexes and parses one directory at once, for packages nothing demands lazily (generated headers).
func (m *Module) loadPackage(dir, pkgPath, fileRel string, std, test bool) error {
	before := len(m.index.order)
	if err := m.indexPackage(dir, pkgPath, fileRel, std, test); err != nil {
		return err
	}
	if len(m.index.order) == before {
		return nil
	}
	return m.parsePackage(m.index.pkgs[m.index.order[before]])
}
