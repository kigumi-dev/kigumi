package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

// writeFile makes dir/name with body, creating dir.
func writeFile(t *testing.T, dir, name string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runModule(t *testing.T, m *driver.Module, args []string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code, err := driver.Run(m, &out, &errOut, args)
	if err != nil {
		t.Fatalf("run: %v (stderr %s)", err, errOut.String())
	}
	return out.String(), errOut.String(), code
}

// TestAliasImportPaths builds one module name at two versions behind two
// aliases and imports both through their alias prefixes.
func TestAliasImportPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "lib1"), "mod.kg", manifest("github.com/acme/lib", ""))
	writeFile(t, filepath.Join(root, "lib1"), "lib.kg", []byte("pub fn one() -> Int {\n    1\n}\n"))
	writeFile(t, filepath.Join(root, "lib2"), "mod.kg", manifest("github.com/acme/lib", ""))
	writeFile(t, filepath.Join(root, "lib2"), "lib.kg", []byte("pub fn one() -> Int {\n    2\n}\n"))
	app := filepath.Join(root, "app")
	writeFile(t, app, "mod.kg", manifest("app", "Require { name: \"github.com/acme/lib\", version: \"v1.0.0\", alias: \"lib1\" }\nRequire { name: \"github.com/acme/lib\", version: \"v1.1.0\", alias: \"lib2\" }\n"))
	writeFile(t, app, "mod.local.kg", []byte("Replace { name: \"github.com/acme/lib\", alias: \"lib1\", path: \"../lib1\" }\nReplace { name: \"github.com/acme/lib\", alias: \"lib2\", path: \"../lib2\" }\n"))
	writeFile(t, app, "main.kg", []byte("import {one as one1} from lib1\nimport {one as one2} from lib2\nprint \"${one1()} ${one2()}\"\n"))

	m, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Selected) != 2 || m.Selected["lib1"].Version != "v1.0.0" || m.Selected["lib2"].Version != "v1.1.0" {
		t.Fatalf("selection: %+v", m.Selected)
	}
	for _, path := range []string{"lib1", "lib2"} {
		p, ok := m.Packages[path]
		if !ok {
			t.Fatalf("alias package %q missing: %v", path, m.Order)
		}
		if p.Module != path {
			t.Fatalf("%s belongs to %q, want its effective alias key", path, p.Module)
		}
		if p.ImportPrefixes["github.com/acme/lib"] != path {
			t.Fatalf("%s import translation: %+v", path, p.ImportPrefixes)
		}
	}
	out, _, code := runModule(t, m, []string{"app"})
	if code != 0 || out != "1 2\n" {
		t.Fatalf("run: exit %d out %q", code, out)
	}
	if _, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler(), NoLocal: true}); err == nil {
		t.Fatal("--no-local must ignore alias Replaces and miss the uncached dependencies")
	}
}

// TestAliasDependencyInternalImports checks that a module imported through an
// alias can still spell its own internal imports with its real name.
func TestAliasDependencyInternalImports(t *testing.T) {
	root := t.TempDir()
	dep := filepath.Join(root, "dep")
	writeFile(t, dep, "mod.kg", manifest("github.com/acme/lib", ""))
	writeFile(t, dep, "lib.kg", []byte("import {deep} from github.com/acme/lib/sub\n\npub fn one() -> Int {\n    deep()\n}\n"))
	writeFile(t, filepath.Join(dep, "sub"), "sub.kg", []byte("pub fn deep() -> Int {\n    7\n}\n"))
	app := filepath.Join(root, "app")
	writeFile(t, app, "mod.kg", manifest("app", "Require { name: \"github.com/acme/lib\", version: \"v1.0.0\", alias: \"lib1\" }\n"))
	writeFile(t, app, "mod.local.kg", []byte("Replace { name: \"github.com/acme/lib\", alias: \"lib1\", path: \"../dep\" }\n"))
	writeFile(t, app, "main.kg", []byte("import {one} from lib1\nprint \"${one()}\"\n"))

	m, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := m.Packages["lib1/sub"]; !ok {
		t.Fatalf("translated internal import did not index lib1/sub: %v", m.Order)
	}
	out, _, code := runModule(t, m, []string{"app"})
	if code != 0 || out != "7\n" {
		t.Fatalf("run: exit %d out %q", code, out)
	}
}

// TestAliasCacheAndLock fills the cache with two versions of one module,
// locks them under aliases, and loads both side by side. It also proves the
// alias lock entry verifies and that a stale hash is refused.
func TestAliasCacheAndLock(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	std, _ := filepath.Abs("../../../std")

	lock := module.Lock{Entries: map[string]module.LockEntry{}}
	put := func(version, alias, body string) {
		dir, err := module.CacheDir(testCompiler(), module.Require{Name: "github.com/acme/lib", Version: version})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, "mod.kg"), manifest("github.com/acme/lib", ""), 0o644)
		os.WriteFile(filepath.Join(dir, "lib.kg"), []byte(body), 0o644)
		os.WriteFile(filepath.Join(dir, module.OriginFileName), []byte("cafebabe\n"), 0o644)
		hash, err := module.TreeHash(dir)
		if err != nil {
			t.Fatal(err)
		}
		lock.Entries[alias] = module.LockEntry{Name: "github.com/acme/lib", Alias: alias, Version: version, Commit: "cafebabe", Hash: hash}
	}
	put("v1.0.0", "lib1", "pub fn one() -> Int {\n    1\n}\n")
	put("v1.1.0", "lib2", "pub fn one() -> Int {\n    2\n}\n")
	if len(lock.Entries) != 2 {
		t.Fatalf("lock setup: %+v", lock.Entries)
	}

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), manifest("app", "Require { name: \"github.com/acme/lib\", version: \"v1.0.0\", alias: \"lib1\" }\nRequire { name: \"github.com/acme/lib\", version: \"v1.1.0\", alias: \"lib2\" }\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import {one as one1} from lib1\nimport {one as one2} from lib2\nprint \"${one1()} ${one2()}\"\n"), 0o644)
	if err := module.WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatalf("load with alias lock: %v", err)
	}
	if m.Selected["lib1"].Version != "v1.0.0" || m.Selected["lib2"].Version != "v1.1.0" {
		t.Fatalf("selection: %+v", m.Selected)
	}
	out, _, code := runModule(t, m, []string{"app"})
	if code != 0 || out != "1 2\n" {
		t.Fatalf("run: exit %d out %q", code, out)
	}

	e := lock.Entries["lib1"]
	e.Hash = "h1:0000000000000000000000000000000000000000000000000000000000000000"
	lock.Entries["lib1"] = e
	if err := module.WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("a stale alias lock hash was not refused: %v", err)
	}
}

// TestNoAliasVersionSelectionIsUnchanged pins the old behavior: the same real
// name without aliases still converges to one selected version and one
// package prefix.
func TestNoAliasVersionSelectionIsUnchanged(t *testing.T) {
	root := t.TempDir()
	dep := filepath.Join(root, "dep")
	writeFile(t, dep, "mod.kg", manifest("github.com/acme/lib", ""))
	writeFile(t, dep, "lib.kg", []byte("pub fn one() -> Int {\n    2\n}\n"))
	app := filepath.Join(root, "app")
	writeFile(t, app, "mod.kg", manifest("app", "Require { name: \"github.com/acme/lib\", version: \"v1.0.0\" }\nRequire { name: \"github.com/acme/lib\", version: \"v1.1.0\" }\n"))
	writeFile(t, app, "mod.local.kg", []byte("Replace { name: \"github.com/acme/lib\", path: \"../dep\" }\n"))
	writeFile(t, app, "main.kg", []byte("import {one} from github.com/acme/lib\nprint \"${one()}\"\n"))

	m, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Selected) != 1 || m.Selected["github.com/acme/lib"].Version != "v1.1.0" {
		t.Fatalf("plain selection changed: %+v", m.Selected)
	}
	if _, ok := m.Packages["github.com/acme/lib"]; !ok {
		t.Fatalf("plain package prefix missing: %v", m.Order)
	}
	out, _, code := runModule(t, m, []string{"app"})
	if code != 0 || out != "2\n" {
		t.Fatalf("run: exit %d out %q", code, out)
	}
}
