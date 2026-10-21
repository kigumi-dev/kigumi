package cli_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestModVerifyFailsOnMissingCache checks that a missing cache directory fails verify instead of printing "not fetched" and exiting 0.
func TestModVerifyFailsOnMissingCache(t *testing.T) {
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
	os.RemoveAll(cached)

	if code, out, _ := execute(t, "mod", "verify", "-C", app); code != 1 {
		t.Fatalf("verify with a missing cache entry must fail (exit 1), got exit %d: %q", code, out)
	}
}

// TestModVerifyFailsOnIncompleteCache checks that a cache directory with no origin marker also fails, since TreeHash alone can't tell it apart.
func TestModVerifyFailsOnIncompleteCache(t *testing.T) {
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
	if err := os.Remove(filepath.Join(cached, ".kigumi-origin")); err != nil {
		t.Fatal(err)
	}

	if code, out, _ := execute(t, "mod", "verify", "-C", app); code != 1 {
		t.Fatalf("verify on a cache missing its origin marker must fail, got exit %d: %q", code, out)
	}
}
