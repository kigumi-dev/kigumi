package driver

import (
	"path/filepath"
	"strings"

	"kigumi/internal/toolchain/module"
)

// skipper skips what is not a package of this module; a nested module is remembered so an import of it can say what it is.
func (m *Module) skipper(root string, opts LoadOptions) func(dir string) bool {
	return func(dir string) bool {
		if module.IgnoredDir(opts.Compiler, dir, opts.StdRoot) {
			return true
		}
		if !module.IsModuleRoot(dir) {
			return false
		}
		if rel, err := filepath.Rel(root, dir); err == nil {
			m.nestedModules = append(m.nestedModules, strings.TrimSuffix(m.Name+"/"+filepath.ToSlash(rel), "/"))
		}
		return true
	}
}
