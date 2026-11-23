package get_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestGetAliases fetches one module at two versions under two aliases, locks
// them separately, verifies both, and runs imports through the aliases.
func TestGetAliases(t *testing.T) {
	root := t.TempDir()
	repo := clitest.Monorepo(t, root)
	clitest.Git(t, repo, "tag", "sgx/v0.2.0")
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app",
		"Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \"v0.1.0\"\n    alias: \"sgx1\"\n    url: \"file://"+repo+"\"\n}\n"+
			"Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \"v0.2.0\"\n    alias: \"sgx2\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {hello as hi1} from sgx1/enclave\nimport {hello as hi2} from sgx2/enclave\nprint \"${hi1()} ${hi2()}\"\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	if code, out, errOut := clitest.Execute(t, "get", app); code != 0 {
		t.Fatalf("get: exit %d out %q err %q", code, out, errOut)
	}
	for _, v := range []string{"v0.1.0", "v0.2.0"} {
		dir := filepath.Join(cache, "kigumi", "mod", "github.com", "kigumilang", "exp", "sgx@"+v)
		if _, err := os.Stat(filepath.Join(dir, "enclave", "enclave.kg")); err != nil {
			t.Fatalf("%s cache entry: %v", v, err)
		}
	}
	lock, err := os.ReadFile(filepath.Join(app, "mod.lock.kg"))
	if err != nil || strings.Count(string(lock), "alias:") != 2 {
		t.Fatalf("lock should record both aliases: %v\n%s", err, lock)
	}
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "hi hi\n" {
		t.Fatalf("run: exit %d out %q err %q", code, out, errOut)
	}
	if code, out, _ := clitest.Execute(t, "mod", "verify", "-C", app); code != 0 || !strings.Contains(out, "sgx1") || !strings.Contains(out, "sgx2") {
		t.Fatalf("verify: exit %d %q", code, out)
	}
}
