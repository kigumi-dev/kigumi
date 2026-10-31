package sem

import (
	"cmp"
	"slices"

	"kigumi/internal/compiler/syntax"
)

// addPackages sorts packages and files here so ids and diagnostics are
// deterministic.
func (r *Result) addPackages(mod *Module) {
	pkgs := slices.Clone(mod.Packages)
	slices.SortStableFunc(pkgs, func(a, b *Package) int {
		return cmp.Or(cmp.Compare(moduleRank(a.Module), moduleRank(b.Module)),
			cmp.Compare(a.Module, b.Module), cmp.Compare(a.Path, b.Path))
	})
	for _, p := range pkgs {
		id := PackageID(len(r.Packages))
		info := PackageInfo{Path: p.Path, Module: p.Module, Std: p.Std}
		info.Scope = r.newScope(ScopePackage, r.prelude, 0, 0)
		files := slices.Clone(p.Files)
		slices.SortStableFunc(files, func(a, b *syntax.Tree) int { return cmp.Compare(a.File.Name, b.File.Name) })
		for _, t := range files {
			fid := FileID(len(r.Files))
			r.Files = append(r.Files, newFileInfo(t, id))
			r.fileIndex[t] = fid
			info.Files = append(info.Files, fid)
			if t == p.Entry {
				info.Entry = fid
			}
		}
		r.Packages = append(r.Packages, info)
		if _, dup := r.pathIndex[p.Path]; !dup {
			r.pathIndex[p.Path] = id
		}
	}
}
