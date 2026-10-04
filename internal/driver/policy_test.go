package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// Development is BuildPolicy's zero value, so every pre-existing call
// site that never mentions Policy keeps today's behavior.
func TestDevelopmentIsZeroValue(t *testing.T) {
	if driver.Development() != (driver.BuildPolicy{}) {
		t.Fatal("Development() must equal the zero BuildPolicy")
	}
	if driver.Development().IsRelease() {
		t.Fatal("Development must not be a release policy")
	}
	if !driver.Release().IsRelease() {
		t.Fatal("Release must be a release policy")
	}
	if got := driver.Development().String(); got != "development" {
		t.Fatalf("Development().String() = %q", got)
	}
	if got := driver.Release().String(); got != "release-v1" {
		t.Fatalf("Release().String() = %q", got)
	}
}

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

	m, err := driver.LoadModule(app, driver.LoadOptions{Policy: driver.Development()})
	if err != nil {
		t.Fatalf("development must still load from the replacement: %v", err)
	}
	report := driver.EvaluateRelease(m, driver.Development())
	if len(report.Warnings) != 1 || report.Warnings[0].ID != "release-local-replace" {
		t.Fatalf("want one release-local-replace warning, got %+v", report)
	}
	if report.HasErrors() {
		t.Fatal("development must not error on a local replacement")
	}

	_, err = driver.LoadModule(app, driver.LoadOptions{Policy: driver.Release()})
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
	dir, err := driver.CacheDir(driver.Require{Name: "greet", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	writeDepModule(t, dir, "greet")
	os.WriteFile(filepath.Join(dir, driver.OriginFileName), []byte("deadbeef\n"), 0o644)

	script := filepath.Join(root, "main.kg")
	os.WriteFile(script, []byte("Kigumi { version: \"0.1\" }\nRequire { name: \"greet\", version: \"v1.0.0\" }\n\nimport {greeting} from greet\nprint greeting()\n"), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Policy: driver.Development()})
	if err != nil {
		t.Fatalf("development must load an unlocked script: %v", err)
	}
	report := driver.EvaluateRelease(m, driver.Development())
	if len(report.Warnings) != 1 || report.Warnings[0].ID != "release-unlocked-script" {
		t.Fatalf("want one release-unlocked-script warning, got %+v", report)
	}

	_, err = driver.LoadModule(root, driver.LoadOptions{Policy: driver.Release()})
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

	m, err := driver.LoadModule(root, driver.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	devReport := driver.EvaluateRelease(m, driver.Development())
	if len(devReport.Warnings) != 1 || devReport.Warnings[0].ID != "release-native-link-unresolved" {
		t.Fatalf("want one release-native-link-unresolved warning, got %+v", devReport)
	}
	relReport := driver.EvaluateRelease(m, driver.Release())
	if len(relReport.Errors) != 1 || relReport.Errors[0].ID != "release-native-link-unresolved" {
		t.Fatalf("want one release-native-link-unresolved error, got %+v", relReport)
	}
}

// TestReportRenderShape checks EvaluateRelease's rendering: a severity word, the
// stable id, detail lines, a release-policy reason, a help line, and a summary.
func TestReportRenderShape(t *testing.T) {
	report := driver.EvaluateRelease(&driver.Module{Violations: []driver.Violation{{
		ID:      "release-local-replace",
		Subject: "github.com/acme/codec is resolved from a local directory",
		Detail:  []driver.KV{{Key: "selected by", Value: "/work/mod.local.kg:3"}, {Key: "replacement", Value: "/work/codec"}},
		Policy:  "rejected because the content is not the locked module tree",
		Help:    "update and lock the dependency, or keep this as a development build",
	}}}, driver.Development())
	out := report.Render()
	for _, want := range []string{
		"warning[release-local-replace]: github.com/acme/codec is resolved from a local directory\n",
		"  selected by: /work/mod.local.kg:3\n",
		"  replacement: /work/codec\n",
		"  release policy: rejected because the content is not the locked module tree\n",
		"  help: update and lock the dependency, or keep this as a development build\n",
		"release readiness: 1 warning(s); would fail with --release\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q in:\n%s", want, out)
		}
	}
}

// Ambient violations only the CLI can detect (KIGUMI_STD, KIGUMI_CFLAGS)
// plug into EvaluateRelease the same way as load-time ones.
func TestAmbientViolationsSeverity(t *testing.T) {
	m := &driver.Module{}
	extra := []driver.Violation{driver.AmbientStdViolation("/env/std"), driver.AmbientCFlagsViolation([]string{"-march=native"})}
	dev := driver.EvaluateRelease(m, driver.Development(), extra...)
	if len(dev.Warnings) != 2 || dev.HasErrors() {
		t.Fatalf("development: want 2 warnings, 0 errors, got %+v", dev)
	}
	rel := driver.EvaluateRelease(m, driver.Release(), extra...)
	if len(rel.Errors) != 2 || len(rel.Warnings) != 0 {
		t.Fatalf("release: want 2 errors, 0 warnings, got %+v", rel)
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
	dir, err := driver.CacheDir(driver.Require{Name: "greet", Version: "v1.0.0"})
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
	if _, err := driver.LoadModule(app, driver.LoadOptions{}); err != nil {
		t.Fatalf("cache matches the lock, should load: %v", err)
	}

	// Add a Link declaration to the dependency's manifest without
	// updating the lock: the tree hash no longer matches.
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"greet\"\n    kigumi: \"0.1\"\n}\nLink { library: \"m\" }\n"), 0o644)

	for _, policy := range []driver.BuildPolicy{driver.Development(), driver.Release()} {
		_, err = driver.LoadModule(app, driver.LoadOptions{Policy: policy})
		if err == nil {
			t.Fatalf("%s: a tampered native declaration should be rejected", policy)
		}
		if !strings.Contains(err.Error(), driver.LockName) {
			t.Fatalf("%s: wrong diagnostic: %v", policy, err)
		}
	}
}
