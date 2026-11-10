package session

import (
	"os"
	"path/filepath"
	"testing"
)

// A package directory inside a module names its entry file, so `kigumi build cmd/app` works next to `cmd/tool`;
// module roots and plain library directories pass through unchanged.
func TestPackageEntry(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"cmd/app/main.kg", "cmd/tool/tool.kg", "lib/a.kg", "lib/b.kg"} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte("print 1\n"), 0o644)
	}
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	for dir, want := range map[string]string{
		"cmd/app":  "cmd/app/main.kg",
		"cmd/tool": "cmd/tool/tool.kg",
		"lib":      "lib",
		".":        ".",
	} {
		got := packageEntry(filepath.Join(root, dir))
		if got != filepath.Join(root, want) {
			t.Errorf("packageEntry(%s) = %s, want %s", dir, got, filepath.Join(root, want))
		}
	}
}

// An ancestor that is only a module through its cmd/ directory (a checkout's Go tooling, say) must not re-root a
// directory passed by absolute path: the target with its own sources is the whole module, exactly as it is when the
// same path is spelled relatively.
func TestPackageEntryIgnoresCmdOnlyAncestor(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"cmd/tool/main.go", "work/pkg/main.kg"} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte("print 1\n"), 0o644)
	}
	dir := filepath.Join(root, "work", "pkg")
	if got := packageEntry(dir); got != dir {
		t.Errorf("packageEntry(%s) = %s, want the directory itself", dir, got)
	}
	// The same tree with a manifest does declare the module, so the package names its entry file.
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	if got, want := packageEntry(dir), filepath.Join(dir, "main.kg"); got != want {
		t.Errorf("packageEntry(%s) = %s, want %s", dir, got, want)
	}
}
