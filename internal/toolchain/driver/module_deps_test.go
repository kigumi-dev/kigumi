package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

const modHead = "Module {\n    name: \"%s\"\n    kigumi: \"0.1\"\n}\n"

func manifest(name, rest string) []byte {
	return []byte(strings.Replace(modHead, "%s", name, 1) + rest)
}

// Requires lib at two versions through two paths, rejects overlapping names, and
// checks the longest-match attribution of nested module names.
func TestDependencyConflict(t *testing.T) {
	root := t.TempDir()
	write := func(dir, name string, body []byte) {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
		os.WriteFile(filepath.Join(root, dir, name), body, 0o644)
	}
	write("lib", "mod.kg", manifest("github.com/acme/lib", ""))
	write("lib", "lib.kg", []byte("pub fn one() -> Int {\n    1\n}\n"))
	write("mid", "mod.kg", manifest("github.com/acme/mid", "Require { name: \"github.com/acme/lib\", version: \"v1.1.0\" }\n"))
	write("mid", "mid.kg", []byte("pub fn two() -> Int {\n    2\n}\n"))
	write("app", "mod.kg", manifest("github.com/acme/app", "Require { name: \"github.com/acme/lib\", version: \"v1.0.0\" }\nRequire { name: \"github.com/acme/mid\", version: \"v0.1.0\" }\n"))
	write("app", "mod.local.kg", []byte("Replace { name: \"github.com/acme/lib\", path: \"../lib\" }\nReplace { name: \"github.com/acme/mid\", path: \"../mid\" }\n"))
	write("app", "main.kg", []byte("import {two} from github.com/acme/mid\nprint \"${two()}\"\n"))
	// Minimal version selection reconciles v1.0.0 (app) with v1.1.0 (mid).
	m0, err := driver.LoadModule(filepath.Join(root, "app"), driver.LoadOptions{Compiler: testCompiler()})
	if err != nil || m0.Selected["github.com/acme/lib"].Version != "v1.1.0" {
		t.Fatalf("selection: %v %+v", err, m0.Selected)
	}
	// A v2 module is a different path from the v1 one (/v2), so both are
	// selected side by side instead of conflicting.
	write("libv2", "mod.kg", manifest("github.com/acme/lib/v2", ""))
	write("libv2", "lib.kg", []byte("pub fn two_lib() -> Int {\n    2\n}\n"))
	write("mid", "mod.kg", manifest("github.com/acme/mid", "Require { name: \"github.com/acme/lib\", version: \"v1.1.0\" }\nRequire { name: \"github.com/acme/lib/v2\", version: \"v2.0.0\" }\n"))
	write("app", "mod.local.kg", []byte("Replace { name: \"github.com/acme/lib\", path: \"../lib\" }\nReplace { name: \"github.com/acme/lib/v2\", path: \"../libv2\" }\nReplace { name: \"github.com/acme/mid\", path: \"../mid\" }\n"))
	m1, err := driver.LoadModule(filepath.Join(root, "app"), driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatalf("a /v2 module must coexist with the unsuffixed one: %v", err)
	}
	if m1.Selected["github.com/acme/lib"].Version != "v1.1.0" || m1.Selected["github.com/acme/lib/v2"].Version != "v2.0.0" {
		t.Fatalf("major-suffixed selection: %+v", m1.Selected)
	}
	// A v2 tag on an unsuffixed path is refused with the path to use instead.
	write("mid", "mod.kg", manifest("github.com/acme/mid", "Require { name: \"github.com/acme/lib\", version: \"v2.0.0\" }\n"))
	if _, err := driver.LoadModule(filepath.Join(root, "app"), driver.LoadOptions{Compiler: testCompiler()}); err == nil || !strings.Contains(err.Error(), "must be github.com/acme/lib/v2") {
		t.Fatalf("a v2 tag without the suffix was not reported: %v", err)
	}
	// A required name nested under the root's own name must not be rejected as an overlap by name alone.
	write("app", "mod.kg", manifest("github.com/acme/app", "Require { name: \"github.com/acme/app/sub\", version: \"v1.0.0\" }\n"))
	m, err := driver.LoadModule(filepath.Join(root, "app"), driver.LoadOptions{Compiler: testCompiler(), NoDeps: true})
	if err != nil {
		t.Fatal(err)
	}
	used := m.ImportedModules(module.ModFile{Requires: []module.Require{{Name: "github.com/acme"}, {Name: "github.com/acme/mid"}}})
	if !used["github.com/acme/mid"] || used["github.com/acme"] {
		t.Fatalf("imported modules should use the longest match: %v", used)
	}
	if _, ok := m.Packages["github.com/acme/app"]; !ok || m.Rel("github.com/acme/app/cmd/x") != "cmd/x" {
		t.Fatalf("own packages carry the module name: %v", m.Order)
	}
}

// A monorepo example nested under its own library loads and builds, while a real
// package-path collision is still rejected.
func TestModuleNamePrefixNotOverlap(t *testing.T) {
	root := t.TempDir()
	write := func(dir, name string, body []byte) {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
		os.WriteFile(filepath.Join(root, dir, name), body, 0o644)
	}
	write("lib", "mod.kg", manifest("example.com/lib", ""))
	write("lib", "lib.kg", []byte("pub fn helper() -> Int {\n    1\n}\n"))
	demo := "lib/examples/demo"
	write(demo, "mod.kg", manifest("example.com/lib/examples/demo", "Require { name: \"example.com/lib\", version: \"v1.0.0\" }\n"))
	write(demo, "mod.local.kg", []byte("Replace { name: \"example.com/lib\", path: \"../..\" }\n"))
	write(demo, "main.kg", []byte("import {helper} from example.com/lib\nprint \"${helper()}\"\n"))
	m, err := driver.LoadModule(filepath.Join(root, demo), driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatalf("example nested under its own library should load: %v", err)
	}
	if _, ok := m.Packages["example.com/lib"]; !ok {
		t.Fatalf("dependency package missing: %v", m.Order)
	}
	var out, errOut strings.Builder
	if code, err := driver.Run(m, &out, &errOut, []string{"app"}); err != nil || code != 0 || out.String() != "1\n" {
		t.Fatalf("run: %v exit %d out %q err %q", err, code, out.String(), errOut.String())
	}

	write("dup", "mod.kg", manifest("example.com/dup", "Require { name: \"example.com/dup/sub\", version: \"v1.0.0\" }\n"))
	write("dup/sub", "sub.kg", []byte("pub fn x() -> Int {\n    1\n}\n"))
	write("dup", "mod.local.kg", []byte("Replace { name: \"example.com/dup/sub\", path: \"../othersub\" }\n"))
	write("othersub", "othersub.kg", []byte("pub fn y() -> Int {\n    2\n}\n"))
	if _, err := driver.LoadModule(filepath.Join(root, "dup"), driver.LoadOptions{Compiler: testCompiler()}); err == nil || !strings.Contains(err.Error(), "overlaps this module's name") {
		t.Fatalf("real overlap not reported: %v", err)
	}
}

// A dependency with no package file in its own root directory must not be rejected
// merely because its bare name coincides with a package path the root already owns.
func TestDependencyEmptyRootNotOverlap(t *testing.T) {
	root := t.TempDir()
	write := func(dir, name string, body []byte) {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
		os.WriteFile(filepath.Join(root, dir, name), body, 0o644)
	}
	write("lib", "mod.kg", manifest("example.com/lib", "Require { name: \"example.com/lib/app\", version: \"v1.0.0\" }\n"))
	write("lib", "mod.local.kg", []byte("Replace { name: \"example.com/lib/app\", path: \"../dep\" }\n"))
	write("lib/app", "app.kg", []byte("pub fn helper() -> Int {\n    1\n}\n"))
	write("lib", "main.kg", []byte("import {deep} from example.com/lib/app/deep\nprint \"${deep()}\"\n"))
	write("dep", "mod.kg", manifest("example.com/lib/app", ""))
	write("dep/deep", "deep.kg", []byte("pub fn deep() -> Int {\n    2\n}\n"))
	m, err := driver.LoadModule(filepath.Join(root, "lib"), driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatalf("dependency with an empty root directory should not overlap: %v", err)
	}
	if _, ok := m.Packages["example.com/lib/app"]; !ok {
		t.Fatalf("root's own app package missing: %v", m.Order)
	}
	if _, ok := m.Packages["example.com/lib/app/deep"]; !ok {
		t.Fatalf("dependency's deep package missing: %v", m.Order)
	}
	var out, errOut strings.Builder
	if code, err := driver.Run(m, &out, &errOut, []string{"lib"}); err != nil || code != 0 || out.String() != "2\n" {
		t.Fatalf("run: %v exit %d out %q err %q", err, code, out.String(), errOut.String())
	}
}

// TestVersionSelection picks the highest version any path asks for and
// reads the manifests of the selected versions only.
func TestVersionSelection(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	lock := module.Lock{Entries: map[string]module.LockEntry{}}
	put := func(name, version, rest string, files map[string]string) {
		dir, _ := module.CacheDir(testCompiler(), module.Require{Name: name, Version: version})
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "mod.kg"), manifest(name, rest), 0o644)
		os.WriteFile(filepath.Join(dir, module.OriginFileName), []byte("cafebabe\n"), 0o644)
		for f, body := range files {
			os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644)
		}
		hash, err := module.TreeHash(dir)
		if err != nil {
			t.Fatal(err)
		}
		lock.Entries[name] = module.LockEntry{Version: version, Commit: "cafebabe", Hash: hash}
	}
	put("github.com/acme/lib", "v1.0.0", "", map[string]string{"lib.kg": "pub fn one() -> Int {\n    1\n}\n"})
	put("github.com/acme/lib", "v1.2.0", "", map[string]string{"lib.kg": "pub fn one() -> Int {\n    2\n}\n"})
	put("github.com/acme/mid", "v0.1.0", "Require { name: \"github.com/acme/lib\", version: \"v1.2.0\" }\n", map[string]string{"mid.kg": "import {one} from github.com/acme/lib\n\npub fn two() -> Int {\n    one() * 10\n}\n"})
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), manifest("app", "Require { name: \"github.com/acme/lib\", version: \"v1.0.0\" }\nRequire { name: \"github.com/acme/mid\", version: \"v0.1.0\" }\n"), 0o644)
	if err := module.WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import {one} from github.com/acme/lib\nimport {two} from github.com/acme/mid\nprint \"${one()} ${two()}\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	if m.Selected["github.com/acme/lib"].Version != "v1.2.0" {
		t.Fatalf("selection: %+v", m.Selected)
	}
	var out, errOut strings.Builder
	if code, err := driver.Run(m, &out, &errOut, []string{"app"}); err != nil || code != 0 || out.String() != "2 20\n" {
		t.Fatalf("run: %v exit %d out %q err %q", err, code, out.String(), errOut.String())
	}
	if c := module.CompareVersions("v1.10.0", "v1.9.9"); c <= 0 {
		t.Errorf("numeric compare: %d", c)
	}
	if c := module.CompareVersions("v2.0.0-rc1", "v2.0.0"); c >= 0 {
		t.Errorf("prerelease should sort first: %d", c)
	}
	put("github.com/acme/lib", "abc1234", "", map[string]string{"lib.kg": "pub fn one() -> Int {\n    3\n}\n"})
	os.WriteFile(filepath.Join(root, "mod.kg"), manifest("app", "Require { name: \"github.com/acme/lib\", version: \"abc1234\" }\nRequire { name: \"github.com/acme/mid\", version: \"v0.1.0\" }\n"), 0o644)
	if _, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std}); err == nil || !strings.Contains(err.Error(), "commit hash cannot be reconciled") {
		t.Fatalf("hash versus tag should be an error: %v", err)
	}
}
