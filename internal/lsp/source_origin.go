package lsp

import (
	"path/filepath"
	"strings"

	"kigumi/internal/driver"
)

// originKind classifies where a file really lives, so a location or edit
// never points at a workspace path the file was not read from.
type originKind string

const (
	originWorkspace originKind = "workspace"
	originStd       originKind = "std"
	originGenerated originKind = "generated"
	originCache     originKind = "cache"
	originLocal     originKind = "local"
)

// fileOrigin is the real path a logical file name (driver.Package file
// name) resolves to, plus whether the LSP may edit it.
type fileOrigin struct {
	path     string
	kind     originKind
	editable bool
}

// buildOrigins maps each file to pkg.Dir plus its base name, since Dir is
// always the real read directory regardless of the logical name's prefix.
func buildOrigins(mod *driver.Module) map[string]fileOrigin {
	out := map[string]fileOrigin{}
	for _, p := range mod.Order {
		pkg := mod.Packages[p]
		kind, editable := classifyPackage(mod, pkg)
		for _, t := range pkg.Files {
			name := t.File.Name
			base := name
			if i := strings.LastIndexByte(base, '/'); i >= 0 {
				base = base[i+1:]
			}
			out[name] = fileOrigin{path: filepath.Join(pkg.Dir, filepath.FromSlash(base)), kind: kind, editable: editable}
		}
	}
	return out
}

func classifyPackage(mod *driver.Module, pkg *driver.Package) (originKind, bool) {
	switch {
	case pkg.Std:
		return originStd, false
	case pkg.Header:
		return originGenerated, false
	case pkg.Module != "":
		if _, ok := mod.LocalReplaceDir(pkg.Module); ok {
			return originLocal, false
		}
		return originCache, false
	default:
		return originWorkspace, true
	}
}

// originOf answers (fileOrigin{}, false) for names buildOrigins never saw:
// a synthetic file, or one the module failed to load.
func (snap *snapshot) originOf(name string) (fileOrigin, bool) {
	o, ok := snap.origins[name]
	return o, ok
}
