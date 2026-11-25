package get_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestGetSemverRangeSelectsAndUpdatesHighestTag(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(repo, "mod.kg"), clitest.Manifest("example.com/dep", ""), 0o644)
	clitest.Git(t, repo, "init", "-q")
	writeDep := func(version string) {
		os.WriteFile(filepath.Join(repo, "dep.kg"), []byte("pub fn value() -> String {\n    \""+version+"\"\n}\n"), 0o644)
		clitest.Git(t, repo, "add", ".")
		clitest.Git(t, repo, "commit", "-q", "-m", version)
		clitest.Git(t, repo, "tag", version)
	}
	writeDep("v1.0.0")
	writeDep("v1.2.0")
	writeDep("v1.4.0")

	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", "Require {\n    name: \"example.com/dep\"\n    version: \"^1.2.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import dep from example.com/dep\nprint dep.value()\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	if code, _, errOut := clitest.Execute(t, "get", app); code != 0 {
		t.Fatalf("get: %s", errOut)
	}
	lockPath := filepath.Join(app, "mod.lock.kg")
	lock, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lock), `version: "v1.4.0"`) || !strings.Contains(string(lock), "commit:") {
		t.Fatalf("range lock did not select v1.4.0:\n%s", lock)
	}
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "v1.4.0\n" {
		t.Fatalf("range run: exit %d out %q err %q", code, out, errOut)
	}
	if code, out, errOut := clitest.Execute(t, "mod", "verify", "-C", app); code != 0 || !strings.Contains(out, "v1.4.0: ok") {
		t.Fatalf("range verify: exit %d out %q err %q", code, out, errOut)
	}

	writeDep("v1.5.0")
	if code, _, errOut := clitest.Execute(t, "get", app); code != 0 {
		t.Fatalf("get without -u: %s", errOut)
	}
	lock, _ = os.ReadFile(lockPath)
	if !strings.Contains(string(lock), `version: "v1.4.0"`) || strings.Contains(string(lock), `version: "v1.5.0"`) {
		t.Fatalf("get without -u moved the range lock:\n%s", lock)
	}

	if code, _, errOut := clitest.Execute(t, "get", "-u", app); code != 0 {
		t.Fatalf("get -u: %s", errOut)
	}
	lock, _ = os.ReadFile(lockPath)
	if !strings.Contains(string(lock), `version: "v1.5.0"`) {
		t.Fatalf("get -u did not select v1.5.0:\n%s", lock)
	}
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "v1.5.0\n" {
		t.Fatalf("range run after -u: exit %d out %q err %q", code, out, errOut)
	}
}

func TestGetAddSemverRangeWritesRangeAndExactLock(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	os.MkdirAll(repo, 0o755)
	os.WriteFile(filepath.Join(repo, "mod.kg"), clitest.Manifest("example.com/dep", ""), 0o644)
	os.WriteFile(filepath.Join(repo, "dep.kg"), []byte("pub fn value() -> Int {\n    1\n}\n"), 0o644)
	clitest.Git(t, repo, "init", "-q")
	clitest.Git(t, repo, "add", ".")
	clitest.Git(t, repo, "commit", "-q", "-m", "v1")
	clitest.Git(t, repo, "tag", "v1.0.0")
	os.WriteFile(filepath.Join(repo, "dep.kg"), []byte("pub fn value() -> Int {\n    2\n}\n"), 0o644)
	clitest.Git(t, repo, "commit", "-q", "-am", "v1.2")
	clitest.Git(t, repo, "tag", "v1.2.0")

	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	if code, _, errOut := clitest.Execute(t, "get", "example.com/dep@^1.0.0", "--url", "file://"+repo, app); code != 0 {
		t.Fatalf("get --add range: %s", errOut)
	}
	mf, _ := os.ReadFile(filepath.Join(app, "mod.kg"))
	if !strings.Contains(string(mf), `version: "^1.0.0"`) {
		t.Fatalf("manifest did not keep the range:\n%s", mf)
	}
	lock, _ := os.ReadFile(filepath.Join(app, "mod.lock.kg"))
	if !strings.Contains(string(lock), `version: "v1.2.0"`) {
		t.Fatalf("lock did not select v1.2.0:\n%s", lock)
	}
}
