package get_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestGetLock fetches a nested module by its tag, checks the lock, catches
// a tampered cache and a moved tag, and fetches a commit hash.
func TestGetLock(t *testing.T) {
	root := t.TempDir()
	repo := clitest.Monorepo(t, root)
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", "Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \"v0.1.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import enclave from github.com/kigumilang/exp/sgx/enclave\nprint enclave.hello()\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	if code, out, errOut := clitest.Execute(t, "get", app); code != 0 {
		t.Fatalf("get: %s%s", out, errOut)
	}
	cached := filepath.Join(cache, "kigumi", "mod", "github.com", "kigumilang", "exp", "sgx@v0.1.0")
	if _, err := os.Stat(filepath.Join(cached, "enclave", "enclave.kg")); err != nil {
		t.Fatalf("subtree not extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cached, "..", "web")); err == nil {
		t.Fatal("only the module's subdirectory belongs in the cache")
	}
	lock, err := os.ReadFile(filepath.Join(app, "mod.lock.kg"))
	if err != nil || !strings.Contains(string(lock), "name: \"github.com/kigumilang/exp/sgx\"") {
		t.Fatalf("lock not written: %v\n%s", err, lock)
	}
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "hi\n" {
		t.Fatalf("run: exit %d out %q err %q", code, out, errOut)
	}
	if code, out, _ := clitest.Execute(t, "mod", "verify", "-C", app); code != 0 || !strings.Contains(out, "ok") {
		t.Fatalf("verify: exit %d %q", code, out)
	}

	os.WriteFile(filepath.Join(cached, "enclave", "enclave.kg"), []byte("pub fn hello() -> String {\n    \"tampered\"\n}\n"), 0o644)
	if code, out, _ := clitest.Execute(t, "mod", "verify", "-C", app); code != 1 || !strings.Contains(out, "MISMATCH") {
		t.Fatalf("verify after tampering: exit %d %q", code, out)
	}
	if code, _, _ := clitest.Execute(t, "get", app); code == 0 {
		t.Fatal("get should reject a cache that differs from the lock")
	}

	os.RemoveAll(cached)
	os.WriteFile(filepath.Join(repo, "sgx", "enclave", "enclave.kg"), []byte("pub fn hello() -> String {\n    \"moved\"\n}\n"), 0o644)
	clitest.Git(t, repo, "commit", "-q", "-am", "move tag")
	clitest.Git(t, repo, "tag", "-f", "sgx/v0.1.0")
	if code, _, _ := clitest.Execute(t, "get", app); code == 0 {
		t.Fatal("get should notice the tag moved")
	}
	if code, _, errOut := clitest.Execute(t, "get", "-u", app); code != 0 {
		t.Fatalf("get -u: %s", errOut)
	}
	if code, out, _ := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "moved\n" {
		t.Fatalf("run after -u: exit %d out %q", code, out)
	}

	hash := clitest.Git(t, repo, "rev-parse", "--short=9", "HEAD")
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", "Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \""+hash+"\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	if code, _, errOut := clitest.Execute(t, "get", app); code != 0 {
		t.Fatalf("get by commit: %s", errOut)
	}
	lock, _ = os.ReadFile(filepath.Join(app, "mod.lock.kg"))
	if !strings.Contains(string(lock), "version: \""+hash+"\"") || strings.Contains(string(lock), "commit: \""+hash+"\"") {
		t.Fatalf("lock should keep the short version and the full commit:\n%s", lock)
	}
	if code, out, _ := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "moved\n" {
		t.Fatalf("run by commit: exit %d out %q", code, out)
	}
}

// TestGetAdd resolves the newest tag of a module path and appends the
// Require; a script gets the records at its top.
func TestGetAdd(t *testing.T) {
	root := t.TempDir()
	repo := clitest.Monorepo(t, root)
	clitest.Git(t, repo, "tag", "web/v0.4.0-rc1")
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if code, out, errOut := clitest.Execute(t, "get", "github.com/kigumilang/exp/web", "--url", "file://"+repo, app); code != 0 || !strings.Contains(out, "added github.com/kigumilang/exp/web v0.3.0") {
		t.Fatalf("get add: exit %d %q %q", code, out, errOut)
	}
	mf, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	if !strings.Contains(string(mf), "version: \"v0.3.0\"") {
		t.Fatalf("manifest after add:\n%s", mf)
	}

	script := filepath.Join(root, "tool.kg")
	os.WriteFile(script, []byte("#!/usr/bin/env kigumi\nimport web from github.com/kigumilang/exp/web\nprint \"${web.serve()}\"\n"), 0o644)
	if code, _, errOut := clitest.Execute(t, "get", "github.com/kigumilang/exp/web@v0.3.0", "--url", "file://"+repo, "--lock", script); code != 0 {
		t.Fatalf("get add script: %s", errOut)
	}
	src, _ := os.ReadFile(script)
	if !strings.HasPrefix(string(src), "#!/usr/bin/env kigumi\nKigumi { version:") {
		t.Fatalf("script records:\n%s", src)
	}
	if _, err := os.Stat(script + ".lock.kg"); err != nil {
		t.Fatal("--lock should write the script's lock")
	}
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", script); code != 0 || out != "1\n" {
		t.Fatalf("run script: exit %d out %q err %q", code, out, errOut)
	}
}
