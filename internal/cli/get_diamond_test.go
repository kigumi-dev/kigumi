package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func requireBlock(name, version, url string) string {
	return "Require {\n    name: \"" + name + "\"\n    version: \"" + version + "\"\n    url: \"" + url + "\"\n}\n"
}

// The root wants dep v1.0.0 and mid wants v1.1.0; both versions must be fetched.
func TestGetDiamondFromEmptyCache(t *testing.T) {
	root := t.TempDir()
	dep := filepath.Join(root, "github.com", "acme", "dep")
	mid := filepath.Join(root, "github.com", "acme", "mid")
	app := filepath.Join(root, "app")
	for _, d := range []string{dep, mid, app} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(dep, "mod.kg"), manifest("github.com/acme/dep", ""), 0o644)
	os.WriteFile(filepath.Join(dep, "dep.kg"), []byte("pub fn v() -> Int {\n    1\n}\n"), 0o644)
	git(t, dep, "init", "-q")
	git(t, dep, "add", ".")
	git(t, dep, "commit", "-q", "-m", "one")
	git(t, dep, "tag", "v1.0.0")
	os.WriteFile(filepath.Join(dep, "dep.kg"), []byte("pub fn v() -> Int {\n    2\n}\n"), 0o644)
	git(t, dep, "commit", "-q", "-am", "two")
	git(t, dep, "tag", "v1.1.0")

	os.WriteFile(filepath.Join(mid, "mod.kg"), manifest("github.com/acme/mid", requireBlock("github.com/acme/dep", "v1.1.0", dep)), 0o644)
	os.WriteFile(filepath.Join(mid, "mid.kg"), []byte("pub fn m() -> Int {\n    3\n}\n"), 0o644)
	git(t, mid, "init", "-q")
	git(t, mid, "add", ".")
	git(t, mid, "commit", "-q", "-m", "mid")
	git(t, mid, "tag", "v1.0.0")

	rootReq := requireBlock("github.com/acme/dep", "v1.0.0", dep) + requireBlock("github.com/acme/mid", "v1.0.0", mid)
	os.WriteFile(filepath.Join(app, "mod.kg"), manifest("app", rootReq), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("print 1\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	done := make(chan int, 1)
	var errOut string
	go func() {
		code, _, e := execute(t, "get", app)
		errOut = e
		done <- code
	}()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("get: %s", errOut)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("get did not finish: a diamond dependency loops")
	}
	for _, v := range []string{"v1.0.0", "v1.1.0"} {
		if _, err := os.Stat(filepath.Join(cache, "kigumi", "mod", "github.com", "acme", "dep@"+v)); err != nil {
			t.Errorf("dep %s missing from the cache: %v", v, err)
		}
	}
}
