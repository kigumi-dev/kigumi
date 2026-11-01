package module

import (
	"os"
	"path/filepath"
)

// IsModuleRoot reports whether dir is a module: it holds a mod.kg manifest
// or a cmd/ directory. A module inside another one is loaded on its own.
func IsModuleRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ManifestName)); err == nil {
		return true
	}
	st, err := os.Stat(filepath.Join(dir, "cmd"))
	return err == nil && st.IsDir()
}

// FindModuleRoot walks up from dir to the nearest module root; ok is false
// when none is found before the file system root.
func FindModuleRoot(dir string) (string, bool) {
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if IsModuleRoot(d) {
			return d, true
		}
		if filepath.Dir(d) == d {
			return "", false
		}
	}
}
