package get_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestGet fetches a dependency from a local git repository tagged with the
// required version, then loads the module that needs it.
func TestGet(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "greet.git")
	os.MkdirAll(repo, 0o755)
	os.WriteFile(filepath.Join(repo, "mod.kg"), []byte("Module {\n    name: \"greet\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "greet.kg"), []byte("pub fn hello(name: String) -> String {\n    \"hello, ${name}\"\n}\n"), 0o644)
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "v0.1.0"}, {"tag", "v0.1.0"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v %s", err, out)
		}
	}
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"greet\", version: \"v0.1.0\", url: \"file://"+repo+"\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {hello} from greet\nprint hello(\"deps\")\n"), 0o644)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	std, _ := filepath.Abs("../../../../std")
	if code, _, _ := clitest.Execute(t, "--std", std, "run", app); code == 0 {
		t.Fatal("run before get should fail")
	}
	if code, out, errOut := clitest.Execute(t, "get", app); code != 0 || !strings.Contains(out, "fetched") {
		t.Fatalf("get: exit %d out %q err %q", code, out, errOut)
	}
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "hello, deps\n" {
		t.Fatalf("run after get: exit %d out %q err %q", code, out, errOut)
	}
	if code, out, _ := clitest.Execute(t, "get", app); code != 0 || strings.Contains(out, "fetched") {
		t.Fatalf("second get should reuse the cache: %q", out)
	}
}
