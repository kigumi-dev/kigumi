package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

func cacheEntry(t *testing.T, name, version string, ready bool) string {
	t.Helper()
	dir, err := module.CacheDir(testCompiler(), module.Require{Name: name, Version: version})
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \""+name+"\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "lib.kg"), []byte("pub fn v() -> Int {\n    1\n}\n"), 0o644)
	if ready {
		os.WriteFile(filepath.Join(dir, module.OriginFileName), []byte("abcdef0\n"), 0o644)
	}
	return dir
}

// A cache entry is used only when it is complete and made of regular
// files: a missing origin marker means an unfinished fetch, and a symlink
// could lead outside the cache.
func TestCacheEntryChecks(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	app := t.TempDir()
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"greet\", version: \"v1.0.0\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {v} from greet\nprint \"${v()}\"\n"), 0o644)

	cacheEntry(t, "greet", "v1.0.0", false)
	if _, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler()}); err == nil || !strings.Contains(err.Error(), "is not fetched") {
		t.Fatalf("an entry without the origin marker must count as not fetched: %v", err)
	}
	dir := cacheEntry(t, "greet", "v1.0.0", true)
	hash, err := module.TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := module.WriteLock(app, module.Lock{Entries: map[string]module.LockEntry{
		"greet": {Version: "v1.0.0", Commit: "abcdef0", Hash: hash},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler()}); err != nil {
		t.Fatalf("a complete entry must load: %v", err)
	}
	if err := os.Symlink("/etc/hostname", filepath.Join(dir, "extra.kg")); err != nil {
		t.Skip(err)
	}
	if _, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler()}); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("a symlink in the cache must be refused: %v", err)
	}
}
