package driver

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// ParseSnapshot records the parsed input of m with content-derived ids. The
// checker keeps its own mutable syntax trees, so the snapshot owns the source
// files and metadata, not the working AST.
func (m *Module) ParseSnapshot() *syntax.ParseSnapshot {
	m.ensureParsed()
	if m.parseSnapshot != nil && m.parsedPackages == len(m.Packages) {
		return m.parseSnapshot
	}
	pkgs := make([]syntax.ParsePackage, 0, len(m.Order))
	for _, path := range m.Order {
		p := m.Packages[path]
		if p == nil {
			continue
		}
		module := p.Module
		if p.Std {
			module = "std"
		}
		files := make([]*token.File, 0, len(p.Files))
		for _, t := range p.Files {
			if t != nil && t.File != nil {
				files = append(files, t.File)
			}
		}
		pkgs = append(pkgs, syntax.ParsePackage{Path: p.Path, Module: module, Std: p.Std, Files: files})
	}
	m.parseSnapshot = syntax.NewParseSnapshot(m.Name, pkgs)
	m.parsedPackages = len(m.Packages)
	return m.parseSnapshot
}

// ModuleID is the content-derived identity of what m read; equal for two
// modules whose files are byte-identical, even when file order differs.
func (m *Module) ModuleID() syntax.ModuleID { return m.ParseSnapshot().ModuleID() }

// PackageID is the content-derived identity of one parsed package.
func (m *Module) PackageID(path string) (syntax.PackageID, bool) {
	return m.ParseSnapshot().PackageID(path)
}
