package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModAddRejectsInvalidVersionImmediately checks that mod add with a malformed version fails immediately, not later at mod json.
func TestModAddRejectsInvalidVersionImmediately(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	if code, _, _ := execute(t, "mod", "init", "-C", app, "github.com/acme/app"); code != 0 {
		t.Fatal("init failed")
	}
	if code, errOut := executeReal(t, "mod", "add", "-C", app, "github.com/acme/dep", "not-semver"); code == 0 {
		t.Fatal("an invalid version must be refused by `mod add`, not accepted and left for `mod json` to catch")
	} else if !strings.Contains(errOut, "not-semver") {
		t.Fatalf("error should name the bad version, got %q", errOut)
	}
	mf, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	if strings.Contains(string(mf), "not-semver") {
		t.Fatalf("the manifest must not have been written with the invalid version:\n%s", mf)
	}
}

// TestModReplaceUpsertsSameName checks that replacing the same module twice updates one Replace record, not two with the later silently winning.
func TestModReplaceUpsertsSameName(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	if code, _, _ := execute(t, "mod", "init", "-C", app, "github.com/acme/app"); code != 0 {
		t.Fatal("init failed")
	}
	if code, _, errOut := execute(t, "mod", "add", "-C", app, "github.com/acme/lib", "v1.0.0"); code != 0 {
		t.Fatalf("add: %s", errOut)
	}
	for _, dir := range []string{"one", "two"} {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
	}
	if code, _, errOut := execute(t, "mod", "replace", "-C", app, "github.com/acme/lib", "../one"); code != 0 {
		t.Fatalf("replace one: %s", errOut)
	}
	if code, _, errOut := execute(t, "mod", "replace", "-C", app, "github.com/acme/lib", "../two"); code != 0 {
		t.Fatalf("replace two: %s", errOut)
	}
	local, err := os.ReadFile(filepath.Join(app, "mod.local.kg"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(local), "Replace") != 1 {
		t.Fatalf("exactly one Replace record must remain, got:\n%s", local)
	}
	if !strings.Contains(string(local), "../two") || strings.Contains(string(local), "../one") {
		t.Fatalf("the Replace record must be upserted to the new path, got:\n%s", local)
	}
}

// TestModTidyRemovesLocalReplace checks that tidy also drops an unused dependency's mod.local.kg Replace, not just its Require and lock entry.
func TestModTidyRemovesLocalReplace(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	if code, _, _ := execute(t, "mod", "init", "-C", app, "github.com/acme/app"); code != 0 {
		t.Fatal("init failed")
	}
	if code, _, errOut := execute(t, "mod", "add", "-C", app, "github.com/acme/unused", "v1.0.0"); code != 0 {
		t.Fatalf("add: %s", errOut)
	}
	os.MkdirAll(filepath.Join(root, "unused"), 0o755)
	if code, _, errOut := execute(t, "mod", "replace", "-C", app, "github.com/acme/unused", "../unused"); code != 0 {
		t.Fatalf("replace: %s", errOut)
	}
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	if code, out, errOut := execute(t, "mod", "tidy", "-C", app); code != 0 || !strings.Contains(out, "removed github.com/acme/unused") {
		t.Fatalf("tidy: exit %d %q %q", code, out, errOut)
	}
	local, err := os.ReadFile(filepath.Join(app, "mod.local.kg"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(local), "acme/unused") {
		t.Fatalf("tidy must drop the Replace for a removed dependency, got:\n%s", local)
	}
}

// TestModInitEmptyManifestRejected checks that a mod.kg with no Module record is an error, not a silent third state besides present/absent.
func TestModInitEmptyManifestRejected(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("// nothing here yet\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	if code, _, errOut := execute(t, "--std", std, "check", root); code == 0 {
		t.Fatalf("a comment-only mod.kg must be rejected, not treated as a valid empty manifest; stderr %q", errOut)
	}
}

func TestModTidyDropsTheCommentOfARemovedRequire(t *testing.T) {
	app := t.TempDir()
	manifestSrc := "Module {\n    name: \"github.com/acme/app\"\n    kigumi: \"0.1\"\n}\n\n// used by the importer\nRequire {\n    name: \"github.com/acme/unused\"\n    version: \"v2.0.0\"\n}\n\n// keeps its comment\nRequire {\n    name: \"github.com/acme/lib\"\n    version: \"v1.0.0\"\n}\n"
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte(manifestSrc), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {one} from github.com/acme/lib\nprint \"${one()}\"\n"), 0o644)
	if code, _, errOut := execute(t, "mod", "tidy", "-C", app); code != 0 {
		t.Fatalf("tidy: exit %d %s", code, errOut)
	}
	got, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	want := "Module {\n    name: \"github.com/acme/app\"\n    kigumi: \"0.1\"\n}\n\n// keeps its comment\nRequire {\n    name: \"github.com/acme/lib\"\n    version: \"v1.0.0\"\n}\n"
	if string(got) != want {
		t.Errorf("manifest after tidy:\n%s\nwant:\n%s", got, want)
	}
}

func TestModReplaceRejectsMissingDirectory(t *testing.T) {
	app := t.TempDir()
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"github.com/acme/app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	if code, _, errOut := execute(t, "mod", "replace", "-C", app, "github.com/acme/lib", "../does-not-exist"); code == 0 {
		t.Fatalf("replace of a missing directory: exit %d %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(app, "mod.local.kg")); err == nil {
		t.Error("mod.local.kg was written for a path that does not exist")
	}
}

func TestModTidyKeepsTheHeadingOfARequireGroup(t *testing.T) {
	app := t.TempDir()
	head := "Module {\n    name: \"github.com/acme/app\"\n    kigumi: \"0.1\"\n}\n\n// dependencies\n"
	manifestSrc := head + "Require {\n    name: \"github.com/acme/unused\"\n    version: \"v2.0.0\"\n}\nRequire {\n    name: \"github.com/acme/lib\"\n    version: \"v1.0.0\"\n}\n"
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte(manifestSrc), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {one} from github.com/acme/lib\nprint \"${one()}\"\n"), 0o644)
	if code, _, errOut := execute(t, "mod", "tidy", "-C", app); code != 0 {
		t.Fatalf("tidy: exit %d %s", code, errOut)
	}
	got, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	want := head + "Require {\n    name: \"github.com/acme/lib\"\n    version: \"v1.0.0\"\n}\n"
	if string(got) != want {
		t.Errorf("manifest after tidy:\n%s\nwant:\n%s", got, want)
	}
}

func TestModReplaceRejectsUnrequiredName(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	os.MkdirAll(filepath.Join(root, "ld"), 0o755)
	os.MkdirAll(app, 0o755)
	if code, _, _ := execute(t, "mod", "init", "-C", app, "github.com/acme/app"); code != 0 {
		t.Fatal("init failed")
	}
	if code, _, _ := execute(t, "mod", "replace", "-C", app, "github.com/acme/nosuchdep", "../ld"); code == 0 {
		t.Fatal("replace of a name that is not a Require succeeded")
	}
	if _, err := os.Stat(filepath.Join(app, "mod.local.kg")); err == nil {
		t.Error("mod.local.kg was written for a name that is not required")
	}
}
