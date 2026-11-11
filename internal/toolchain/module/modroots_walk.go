package module

import (
	"io/fs"
	"path/filepath"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
)

// Roots lists workspace and every nested module root below it.
func Roots(o buildconfig.CompilerOptions, workspace, stdRoot string) []string {
	roots := []string{workspace}
	filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == workspace {
			return nil
		}
		if IgnoredDir(o, path, stdRoot) {
			return fs.SkipDir
		}
		if IsModuleRoot(path) {
			roots = append(roots, path)
		}
		return nil
	})
	return roots
}

// IgnoredDir excludes hidden directories, node_modules, a vendor tree and
// the std root from a module walk. Vendor modules are loaded from
// vendor/modules.txt, never as packages of the enclosing module.
func IgnoredDir(o buildconfig.CompilerOptions, path, stdRoot string) bool {
	name := filepath.Base(path)
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == VendorDirName || SameDir(o, path, stdRoot)
}

// SameDir reports whether a and b name the same directory once both are made absolute against o.
func SameDir(o buildconfig.CompilerOptions, a, b string) bool {
	if b == "" {
		return false
	}
	aa, errA := o.Abs(a)
	bb, errB := o.Abs(b)
	return errA == nil && errB == nil && aa == bb
}
