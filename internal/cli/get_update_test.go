package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetUpdateRefreshesExistingCache checks that get -u refetches a moved tag even when the dependency is already cached.
func TestGetUpdateRefreshesExistingCache(t *testing.T) {
	root := t.TempDir()
	repo := monorepo(t, root)
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), manifest("app", "Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \"v0.1.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import enclave from github.com/kigumilang/exp/sgx/enclave\nprint enclave.hello()\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	if code, _, errOut := execute(t, "get", app); code != 0 {
		t.Fatalf("get: %s", errOut)
	}
	cached := filepath.Join(cache, "kigumi", "mod", "github.com", "kigumilang", "exp", "sgx@v0.1.0")
	if _, err := os.Stat(cached); err != nil {
		t.Fatalf("cache entry missing before the test even starts: %v", err)
	}

	// Cache entry is left as-is (unlike TestGetLock) to exercise the "already cached" branch of get -u, not "missing".
	os.WriteFile(filepath.Join(repo, "sgx", "enclave", "enclave.kg"), []byte("pub fn hello() -> String {\n    \"moved\"\n}\n"), 0o644)
	git(t, repo, "commit", "-q", "-am", "move tag")
	git(t, repo, "tag", "-f", "sgx/v0.1.0")

	if code, out, _ := execute(t, "mod", "verify", "-C", app); code != 0 || out == "" {
		t.Fatalf("the existing cache must still satisfy its own (stale) lock: exit %d %q", code, out)
	}
	if code, _, errOut := execute(t, "get", "-u", app); code != 0 {
		t.Fatalf("get -u with an existing cache: %s", errOut)
	}
	std, _ := filepath.Abs("../../std")
	if code, out, errOut := execute(t, "--std", std, "run", app); code != 0 || out != "moved\n" {
		t.Fatalf("run after -u on an existing cache: exit %d out %q err %q", code, out, errOut)
	}
}

// TestGetUpdateAnnotatedTagStaysUpToDate checks that get -u compares an annotated tag's dereferenced commit, not its own SHA, against the lock.
func TestGetUpdateAnnotatedTagStaysUpToDate(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "github.com", "kigumilang", "exp", "sgx")
	os.MkdirAll(repo, 0o755)
	os.WriteFile(filepath.Join(repo, "mod.kg"), manifest("github.com/kigumilang/exp/sgx", ""), 0o644)
	os.WriteFile(filepath.Join(repo, "enclave.kg"), []byte("pub fn hello() -> String {\n    \"hi\"\n}\n"), 0o644)
	git(t, repo, "init", "-q")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "initial")
	git(t, repo, "tag", "-a", "v0.1.0", "-m", "release")

	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), manifest("app", "Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \"v0.1.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	if code, _, errOut := execute(t, "get", app); code != 0 {
		t.Fatalf("get: %s", errOut)
	}
	for i := 0; i < 2; i++ {
		code, out, errOut := execute(t, "get", "-u", app)
		if code != 0 {
			t.Fatalf("get -u (run %d): %s", i, errOut)
		}
		if strings.Contains(out, "moved from") {
			t.Fatalf("get -u (run %d) treated an unmoved annotated tag as moved: %q", i, out)
		}
		if !strings.Contains(out, "up to date") {
			t.Fatalf("get -u (run %d) want up to date, got %q", i, out)
		}
	}
}
