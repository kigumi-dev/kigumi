package driver_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

var stdSysRows = []struct{ triple, sys string }{
	{"x86_64-linux-musl", "posix"},
	{"x86_64-linux-musl", "none"},
	{"aarch64-linux-musl", "none"},
	{"x86_64-freestanding", ""},
	{"x86_64-uefi", ""},
	{"wasm32-freestanding", ""},
	{"wasm32-wasi", "posix"},
	{"wasm32-wasi", "none"},
	{"x86_64-freebsd", "posix"},
	{"x86_64-netbsd", "posix"},
	{"x86_64-openbsd", "posix"},
	{"aarch64-macos", "posix"},
	{"x86_64-windows-gnu", "posix"},
}

// Every std package is checked whatever the entry imports, so a hosted-only name
// outside a `_hosted.kg` file fails here even when no program reaches it.
func TestStdCleanUnderEachSys(t *testing.T) {
	t.Parallel()
	std, _ := filepath.Abs("../../../std")
	dirs := stdPackageDirs(t, std)
	for _, row := range stdSysRows {
		t.Run(row.triple+"/sys="+row.sys, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"stdwalk\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte("fn main() -> Void! {}\n"), 0o644)
			target := mustTarget(t, row.triple, row.sys)
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
			if err != nil {
				t.Fatal(err)
			}
			res, err := driver.Check(m)
			if err != nil {
				t.Fatal(err)
			}
			if out := res.Render(); out != "" {
				t.Errorf("std is not clean for %s with sys %q:\n%s", row.triple, target.Sys, out)
			}
			checked := map[string]bool{}
			for _, p := range res.Packages {
				checked[p.Path] = p.Std
			}
			for _, d := range dirs {
				if !checked[d] {
					t.Errorf("std package %s was not checked", d)
				}
			}
		})
	}
}

func stdPackageDirs(t *testing.T, std string) []string {
	t.Helper()
	seen := map[string]bool{}
	var dirs []string
	err := filepath.WalkDir(std, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kg") {
			return err
		}
		rel, _ := filepath.Rel(std, filepath.Dir(path))
		dir := "std/" + filepath.ToSlash(rel)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}
