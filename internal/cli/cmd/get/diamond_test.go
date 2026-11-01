package get_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/cli/clitest"
)

func requireBlock(name, version, url string) string {
	return "Require {\n    name: \"" + name + "\"\n    version: \"" + version + "\"\n    url: \"file://" + url + "\"\n}\n"
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
	os.WriteFile(filepath.Join(dep, "mod.kg"), clitest.Manifest("github.com/acme/dep", ""), 0o644)
	os.WriteFile(filepath.Join(dep, "dep.kg"), []byte("pub fn v() -> Int {\n    1\n}\n"), 0o644)
	clitest.Git(t, dep, "init", "-q")
	clitest.Git(t, dep, "add", ".")
	clitest.Git(t, dep, "commit", "-q", "-m", "one")
	clitest.Git(t, dep, "tag", "v1.0.0")
	os.WriteFile(filepath.Join(dep, "dep.kg"), []byte("pub fn v() -> Int {\n    2\n}\n"), 0o644)
	clitest.Git(t, dep, "commit", "-q", "-am", "two")
	clitest.Git(t, dep, "tag", "v1.1.0")

	os.WriteFile(filepath.Join(mid, "mod.kg"), clitest.Manifest("github.com/acme/mid", requireBlock("github.com/acme/dep", "v1.1.0", dep)), 0o644)
	os.WriteFile(filepath.Join(mid, "mid.kg"), []byte("pub fn m() -> Int {\n    3\n}\n"), 0o644)
	clitest.Git(t, mid, "init", "-q")
	clitest.Git(t, mid, "add", ".")
	clitest.Git(t, mid, "commit", "-q", "-m", "mid")
	clitest.Git(t, mid, "tag", "v1.0.0")

	rootReq := requireBlock("github.com/acme/dep", "v1.0.0", dep) + requireBlock("github.com/acme/mid", "v1.0.0", mid)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", rootReq), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("print 1\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	done := make(chan int, 1)
	var errOut string
	go func() {
		code, _, e := clitest.Execute(t, "get", app)
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
