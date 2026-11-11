package mod_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestModCommands(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "lib")
	os.MkdirAll(lib, 0o755)
	os.WriteFile(filepath.Join(lib, "mod.kg"), clitest.Manifest("github.com/acme/lib", ""), 0o644)
	os.WriteFile(filepath.Join(lib, "lib.kg"), []byte("pub fn one() -> Int {\n    1\n}\n"), 0o644)
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	if code, _, errOut := clitest.Execute(t, "mod", "init", "-C", app, "github.com/acme/app"); code != 0 || !strings.Contains(errOut, "mod.kg") {
		t.Fatalf("init: exit %d %q", code, errOut)
	}
	if code, _, _ := clitest.Execute(t, "mod", "init", "-C", app); code != 2 {
		t.Fatalf("second init should be a usage error, exit %d", code)
	}
	if code, _, _ := clitest.Execute(t, "mod", "add", "-C", app, "lib", "v1.0.0"); code != 2 {
		t.Fatalf("add of a short name without --url should be a usage error, exit %d", code)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "add", "-C", app, "github.com/acme/lib", "v1.0.0"); code != 0 {
		t.Fatalf("add: %s", errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "replace", "-C", app, "github.com/acme/lib", "../lib"); code != 0 {
		t.Fatalf("replace: %s", errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "add", "-C", app, "github.com/acme/unused/v2", "v2.0.0"); code != 0 {
		t.Fatalf("add: %s", errOut)
	}
	mf, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	if !strings.Contains(string(mf), "kigumi: \"0.1\"") || strings.Contains(string(mf), "Replace") || !strings.Contains(string(mf), "acme/unused/v2") {
		t.Fatalf("manifest after add:\n%s", mf)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "add", "-C", app, "github.com/acme/other", "v2.0.0"); code != 2 || !strings.Contains(errOut, "/v2") {
		t.Fatalf("a v2 tag on an unsuffixed path must be a usage error naming /v2, exit %d: %s", code, errOut)
	}
	if local, err := os.ReadFile(filepath.Join(app, "mod.local.kg")); err != nil || !strings.Contains(string(local), "path: \"../lib\"") {
		t.Fatalf("local override: %v\n%s", err, local)
	}
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {one} from github.com/acme/lib\nprint \"${one()}\"\n"), 0o644)
	if code, out, errOut := clitest.Execute(t, "mod", "tidy", "-C", app); code != 0 || !strings.Contains(errOut, "removed github.com/acme/unused/v2") {
		t.Fatalf("tidy: exit %d %q %q", code, out, errOut)
	}
	if code, out, _ := clitest.Execute(t, "mod", "json", "-C", app); code != 0 || !strings.Contains(out, "\"local\"") || !strings.Contains(out, "acme/lib") {
		t.Fatalf("json: exit %d %q", code, out)
	}
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "1\n" {
		t.Fatalf("run: exit %d out %q err %q", code, out, errOut)
	}
	if code, _, _ := clitest.Execute(t, "--std", std, "--no-local", "run", app); code == 0 {
		t.Fatal("--no-local must ignore the override and miss the dependency")
	}
	if code, out, _ := clitest.Execute(t, "--version"); code != 0 || !strings.HasPrefix(out, "kigumi version ") {
		t.Fatalf("version: exit %d %q", code, out)
	}
}
