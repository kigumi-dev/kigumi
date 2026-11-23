package mod_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestModAliasCommands adds and replaces one module name under two aliases,
// then checks json, tidy and an alias import run.
func TestModAliasCommands(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"lib1", "lib2"} {
		full := filepath.Join(root, dir)
		os.MkdirAll(full, 0o755)
		os.WriteFile(filepath.Join(full, "mod.kg"), clitest.Manifest("github.com/acme/lib", ""), 0o644)
		body := "pub fn one() -> Int {\n    1\n}\n"
		if dir == "lib2" {
			body = "pub fn one() -> Int {\n    2\n}\n"
		}
		os.WriteFile(filepath.Join(full, "lib.kg"), []byte(body), 0o644)
	}
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	if code, _, errOut := clitest.Execute(t, "mod", "init", "-C", app, "app"); code != 0 {
		t.Fatalf("init: %s", errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "add", "-C", app, "github.com/acme/lib", "v1.0.0", "--url", "file:///nowhere", "--alias", "lib1"); code != 0 {
		t.Fatalf("add lib1: %s", errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "add", "-C", app, "github.com/acme/lib", "v1.1.0", "--url", "file:///nowhere", "--alias", "lib2"); code != 0 {
		t.Fatalf("add lib2: %s", errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "replace", "-C", app, "--alias", "lib1", "github.com/acme/lib", "../lib1"); code != 0 {
		t.Fatalf("replace lib1: %s", errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "replace", "-C", app, "--alias", "lib2", "github.com/acme/lib", "../lib2"); code != 0 {
		t.Fatalf("replace lib2: %s", errOut)
	}
	mf, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	if strings.Count(string(mf), "alias: \"lib") != 2 {
		t.Fatalf("manifest aliases:\n%s", mf)
	}
	local, _ := os.ReadFile(filepath.Join(app, "mod.local.kg"))
	if strings.Count(string(local), "alias: \"lib") != 2 {
		t.Fatalf("local aliases:\n%s", local)
	}
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {one as one1} from lib1\nimport {one as one2} from lib2\nprint \"${one1()} ${one2()}\"\n"), 0o644)
	if code, out, _ := clitest.Execute(t, "mod", "json", "-C", app); code != 0 || !strings.Contains(out, "\"alias\": \"lib1\"") || !strings.Contains(out, "\"alias\": \"lib2\"") {
		t.Fatalf("json: exit %d %q", code, out)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "tidy", "-C", app); code != 0 {
		t.Fatalf("tidy: %s", errOut)
	}
	mf, _ = os.ReadFile(filepath.Join(app, "mod.kg"))
	if strings.Count(string(mf), "alias: \"lib") != 2 {
		t.Fatalf("tidy dropped a used alias:\n%s", mf)
	}
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "1 2\n" {
		t.Fatalf("run: exit %d out %q err %q", code, out, errOut)
	}
	if code, _, _ := clitest.Execute(t, "--std", std, "--no-local", "run", app); code == 0 {
		t.Fatal("--no-local must ignore alias Replaces and miss the cached dependencies")
	}
}
