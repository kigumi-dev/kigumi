package get_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func commitTagged(t *testing.T, repo, tag string) {
	clitest.Git(t, repo, "init", "-q")
	clitest.Git(t, repo, "add", ".")
	clitest.Git(t, repo, "commit", "-q", "-m", tag)
	clitest.Git(t, repo, "tag", tag)
}

func TestGetRefusesOptionLikeURL(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	for i, evilTmpl := range []string{"--upload-pack=touch %s", "ext::sh -c touch% %s", "-oProxyCommand=touch %s"} {
		mark := filepath.Join(root, "MARK"+string(rune('a'+i)))
		evil := strings.Replace(evilTmpl, "%s", mark, 1)
		app := filepath.Join(root, "app"+string(rune('a'+i)))
		os.MkdirAll(app, 0o755)
		req := "Require {\n    name: \"example.com/evil\"\n    version: \"v1.0.0\"\n    url: \"" + evil + "\"\n}\n"
		os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", req), 0o644)
		code, _, errOut := clitest.Execute(t, "get", app)
		if code == 0 || !strings.Contains(errOut, "refusing repository url") || !strings.Contains(errOut, evil) {
			t.Errorf("%q: exit %d, stderr %q", evil, code, errOut)
		}
		if _, err := os.Stat(mark); err == nil {
			t.Errorf("%q ran a command", evil)
		}
	}
}

func TestGetRefusesOptionLikeURLFromDependency(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	mark := filepath.Join(root, "MARK")
	evil := "--upload-pack=touch " + mark
	mid := filepath.Join(root, "mid")
	os.MkdirAll(mid, 0o755)
	req := "Require {\n    name: \"example.com/evil\"\n    version: \"v1.0.0\"\n    url: \"" + evil + "\"\n}\n"
	os.WriteFile(filepath.Join(mid, "mod.kg"), clitest.Manifest("example.com/mid", req), 0o644)
	commitTagged(t, mid, "v1.0.0")
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", requireBlock("example.com/mid", "v1.0.0", mid)), 0o644)
	code, _, errOut := clitest.Execute(t, "get", app)
	if code == 0 || !strings.Contains(errOut, evil) {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("a transitive url ran a command")
	}
}

func TestGetRefusesSymlinkedSubdir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	outside := filepath.Join(root, "outside")
	os.MkdirAll(filepath.Join(outside, "inner"), 0o755)
	os.WriteFile(filepath.Join(outside, "inner", "mod.kg"), clitest.Manifest("example.com/repo/sub/inner", ""), 0o644)
	os.WriteFile(filepath.Join(outside, "inner", "keep.txt"), []byte("keep"), 0o644)
	repo := filepath.Join(root, "example.com", "repo")
	os.MkdirAll(repo, 0o755)
	os.WriteFile(filepath.Join(repo, "README"), []byte("x"), 0o644)
	if err := os.Symlink(outside, filepath.Join(repo, "sub")); err != nil {
		t.Skip(err)
	}
	commitTagged(t, repo, "sub/inner/v1.0.0")
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	req := "Require {\n    name: \"example.com/repo/sub/inner\"\n    version: \"v1.0.0\"\n    url: \"file://" + repo + "\"\n}\n"
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", req), 0o644)
	code, _, errOut := clitest.Execute(t, "get", app)
	if code == 0 || !strings.Contains(errOut, "example.com/repo/sub/inner") || !strings.Contains(errOut, "outside") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, f := range []string{"mod.kg", "keep.txt"} {
		if _, err := os.Stat(filepath.Join(outside, "inner", f)); err != nil {
			t.Errorf("%s outside the cache was moved or removed: %v", f, err)
		}
	}
}

func greetRepo(t *testing.T, dir string) {
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "mod.kg"), clitest.Manifest("greet", ""), 0o644)
	os.WriteFile(filepath.Join(dir, "greet.kg"), []byte("pub fn hello(name: String) -> String {\n    \"hello, ${name}\"\n}\n"), 0o644)
	commitTagged(t, dir, "v1.0.0")
}

func TestLockRecordsUrlAndBuildChecksIt(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	repo, fork := filepath.Join(root, "repo"), filepath.Join(root, "fork")
	greetRepo(t, repo)
	greetRepo(t, fork)
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {hello} from greet\nprint hello(\"x\")\n"), 0o644)
	req := func(url string) {
		os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", requireBlock("greet", "v1.0.0", url)), 0o644)
	}
	req(repo)
	if code, _, errOut := clitest.Execute(t, "get", app); code != 0 {
		t.Fatalf("get: %s", errOut)
	}
	lockPath := filepath.Join(app, "mod.lock.kg")
	lock, _ := os.ReadFile(lockPath)
	if !strings.Contains(string(lock), `url: "file://`+repo+`"`) {
		t.Fatalf("lock does not record the url:\n%s", lock)
	}
	std, _ := filepath.Abs("../../../../std")
	if code, _, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 {
		t.Fatalf("run with a matching lock: %s", errOut)
	}
	req(fork)
	if code, _, errOut := clitest.Execute(t, "get", app); code == 0 || !strings.Contains(errOut, "lock recorded") {
		t.Fatalf("get with a changed url: exit %d, %s", code, errOut)
	}
	if code, _, errOut := clitest.Execute(t, "--std", std, "run", app); code == 0 || !strings.Contains(errOut, "locked") {
		t.Fatalf("run with a changed url: exit %d, %s", code, errOut)
	}
	if code, _, errOut := clitest.Execute(t, "get", "-u", app); code != 0 {
		t.Fatalf("get -u accepts the new source: %s", errOut)
	}
	if code, _, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 {
		t.Fatalf("run after get -u: %s", errOut)
	}
}
