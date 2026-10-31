package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/policy"
)

func writeDepModule(t *testing.T, dir, name string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \""+name+"\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, name+".kg"), []byte("pub fn greeting() -> String {\n    \"hi\"\n}\n"), 0o644)
}

// TestLocalReplaceReleasePolicy checks that a mod.local.kg replacement is a warning
// under Development and a build error under Release, even matching the locked tree.
func TestLocalReplaceReleasePolicy(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	dep := filepath.Join(root, "dep")
	os.MkdirAll(app, 0o755)
	writeDepModule(t, dep, "greet")
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"greet\", version: \"v1.0.0\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "mod.local.kg"), []byte("Replace { name: \"greet\", path: \""+filepath.ToSlash(dep)+"\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {greeting} from greet\nprint greeting()\n"), 0o644)

	m, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler(), Policy: policy.Development()})
	if err != nil {
		t.Fatalf("development must still load from the replacement: %v", err)
	}
	report := policy.Evaluate(m.Violations, policy.Development())
	if len(report.Warnings) != 1 || report.Warnings[0].ID != "release-local-replace" {
		t.Fatalf("want one release-local-replace warning, got %+v", report)
	}
	if report.HasErrors() {
		t.Fatal("development must not error on a local replacement")
	}

	_, err = driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler(), Policy: policy.Release()})
	if err == nil {
		t.Fatal("release must refuse a local replacement")
	}
	if !strings.Contains(err.Error(), "release-local-replace") || !strings.Contains(err.Error(), "mod.local.kg") {
		t.Fatalf("wrong diagnostic: %v", err)
	}
}

// A dependency-bearing script without a sidecar lock is a warning under
// Development and a build error under Release, sharing the same id.
func TestUnlockedScriptReleasePolicy(t *testing.T) {
	root := t.TempDir()
	dep := filepath.Join(root, "greet")
	writeDepModule(t, dep, "greet")
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	dir, err := driver.CacheDir(testCompiler(), driver.Require{Name: "greet", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	writeDepModule(t, dir, "greet")
	os.WriteFile(filepath.Join(dir, driver.OriginFileName), []byte("deadbeef\n"), 0o644)

	script := filepath.Join(root, "main.kg")
	os.WriteFile(script, []byte("Kigumi { version: \"0.1\" }\nRequire { name: \"greet\", version: \"v1.0.0\" }\n\nimport {greeting} from greet\nprint greeting()\n"), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), Policy: policy.Development()})
	if err != nil {
		t.Fatalf("development must load an unlocked script: %v", err)
	}
	report := policy.Evaluate(m.Violations, policy.Development())
	if len(report.Warnings) != 1 || report.Warnings[0].ID != "release-unlocked-script" {
		t.Fatalf("want one release-unlocked-script warning, got %+v", report)
	}

	_, err = driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), Policy: policy.Release()})
	if err == nil {
		t.Fatal("release must refuse a script with no sidecar lock")
	}
	if !strings.Contains(err.Error(), "release-unlocked-script") {
		t.Fatalf("wrong diagnostic: %v", err)
	}
}

// A dependency's Link search path that no longer exists on disk is a
// warning under Development and a build error under Release.
func TestNativeLinkUnresolvedReleasePolicy(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nLink { library: \"foo\", search: \"libs\" }\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatal(err)
	}
	devReport := policy.Evaluate(m.Violations, policy.Development())
	if len(devReport.Warnings) != 1 || devReport.Warnings[0].ID != "release-native-link-unresolved" {
		t.Fatalf("want one release-native-link-unresolved warning, got %+v", devReport)
	}
	relReport := policy.Evaluate(m.Violations, policy.Release())
	if len(relReport.Errors) != 1 || relReport.Errors[0].ID != "release-native-link-unresolved" {
		t.Fatalf("want one release-native-link-unresolved error, got %+v", relReport)
	}
}

// TestLoadRejectsTamperedNativeDeclaration checks that changing a dependency's native
// Link/Source declarations without updating the lock is rejected as tampering.
func TestLoadRejectsTamperedNativeDeclaration(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"greet\", version: \"v1.0.0\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {greeting} from greet\nprint greeting()\n"), 0o644)

	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	dir, err := driver.CacheDir(testCompiler(), driver.Require{Name: "greet", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	writeDepModule(t, dir, "greet")
	os.WriteFile(filepath.Join(dir, driver.OriginFileName), []byte("deadbeef\n"), 0o644)

	hash, err := driver.TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.WriteLock(app, driver.Lock{Entries: map[string]driver.LockEntry{
		"greet": {Version: "v1.0.0", Commit: "deadbeef", Hash: hash},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler()}); err != nil {
		t.Fatalf("cache matches the lock, should load: %v", err)
	}

	// Add a Link declaration to the dependency's manifest without
	// updating the lock: the tree hash no longer matches.
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"greet\"\n    kigumi: \"0.1\"\n}\nLink { library: \"m\" }\n"), 0o644)

	for _, pol := range []policy.BuildPolicy{policy.Development(), policy.Release()} {
		_, err = driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler(), Policy: pol})
		if err == nil {
			t.Fatalf("%s: a tampered native declaration should be rejected", pol)
		}
		if !strings.Contains(err.Error(), driver.LockName) {
			t.Fatalf("%s: wrong diagnostic: %v", pol, err)
		}
	}
}
