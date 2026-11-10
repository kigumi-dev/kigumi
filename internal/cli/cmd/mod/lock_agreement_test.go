package mod_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

type world struct {
	t     *testing.T
	root  string
	app   string
	repos string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "xdg"))
	t.Setenv("KIGUMI_PKG_CACHE", filepath.Join(root, "pkg"))
	t.Setenv("KIGUMI_LOCAL", "")
	w := &world{t: t, root: root, app: filepath.Join(root, "app"), repos: filepath.Join(root, "repos")}
	os.MkdirAll(w.app, 0o755)
	return w
}

func (w *world) require(name, version string) string {
	return "Require {\n    name: \"example.com/" + name + "\"\n    version: \"" + version + "\"\n    url: \"file://" + filepath.Join(w.repos, name) + "\"\n}\n"
}

func (w *world) write(path, body string) {
	w.t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		w.t.Fatal(err)
	}
}

// repo publishes versions of example.com/<name>; each is its own commit and tag, with v() returning the value given.
func (w *world) repo(name string, versions map[string][2]string) {
	w.t.Helper()
	dir := filepath.Join(w.repos, name)
	os.MkdirAll(dir, 0o755)
	clitest.Git(w.t, dir, "init", "-q")
	for _, v := range []string{"v1.0.0", "v1.1.0", "v1.2.0"} {
		spec, ok := versions[v]
		if !ok {
			continue
		}
		w.write(filepath.Join(dir, "mod.kg"), string(clitest.Manifest("example.com/"+name, spec[0])))
		w.write(filepath.Join(dir, name+".kg"), "pub fn v() -> Int {\n    "+spec[1]+"\n}\n")
		clitest.Git(w.t, dir, "add", "-A")
		clitest.Git(w.t, dir, "commit", "-q", "-m", v)
		clitest.Git(w.t, dir, "tag", v)
	}
}

func (w *world) kigumi(args ...string) (int, string, string) {
	w.t.Helper()
	return clitest.Execute(w.t, args...)
}

func (w *world) read(rel string) string {
	b, _ := os.ReadFile(filepath.Join(w.app, rel))
	return string(b)
}

// diamond: the root asks for dep v1.0.0 and mid v1.0.0, mid asks for dep v1.1.0, and only dep is imported.
func (w *world) diamond() {
	w.repo("dep", map[string][2]string{"v1.0.0": {"", "100"}, "v1.1.0": {"", "110"}})
	w.repo("mid", map[string][2]string{"v1.0.0": {w.require("dep", "v1.1.0"), "1"}})
	w.write(filepath.Join(w.app, "mod.kg"), string(clitest.Manifest("app", w.require("dep", "v1.0.0")+w.require("mid", "v1.0.0"))))
	w.write(filepath.Join(w.app, "main.kg"), "import dep from example.com/dep\nprint \"${dep.v()}\"\n")
	if code, _, e := w.kigumi("get", w.app); code != 0 {
		w.t.Fatalf("get: %s", e)
	}
}

func TestModTidyListsASelectionChangeAndStops(t *testing.T) {
	w := newWorld(t)
	w.diamond()
	lock, manifest := w.read("mod.lock.kg"), w.read("mod.kg")
	if !strings.Contains(lock, "v1.1.0") {
		t.Fatalf("setup: dep should be locked at v1.1.0:\n%s", lock)
	}
	code, _, e := w.kigumi("mod", "tidy", "-C", w.app)
	if code != 1 {
		t.Fatalf("tidy: exit %d: %s", code, e)
	}
	if !strings.Contains(e, "selection changes: example.com/dep v1.1.0 -> v1.0.0") {
		t.Fatalf("the change must be listed: %s", e)
	}
	if w.read("mod.lock.kg") != lock || w.read("mod.kg") != manifest {
		t.Fatal("a refused tidy must change nothing")
	}
}

func TestModTidyUpdateDropsTheEntryAndLeavesLockingToGet(t *testing.T) {
	w := newWorld(t)
	w.diamond()
	if code, _, e := w.kigumi("mod", "tidy", "-C", w.app, "--update"); code != 0 {
		t.Fatalf("tidy --update: %s", e)
	}
	if strings.Contains(w.read("mod.kg"), "example.com/mid") || strings.Contains(w.read("mod.lock.kg"), "example.com/dep") {
		t.Fatalf("the Require and the stale entry must go:\n%s\n%s", w.read("mod.kg"), w.read("mod.lock.kg"))
	}
	if code, out, _ := w.kigumi("mod", "verify", "-C", w.app); code != 1 || !strings.Contains(out, "selected but not in the lock") {
		t.Fatalf("verify must say the new version is unlocked: %d %q", code, out)
	}
	if code, _, e := w.kigumi("get", w.app); code != 0 {
		t.Fatalf("get: %s", e)
	}
	if code, out, _ := w.kigumi("mod", "verify", "-C", w.app); code != 0 {
		t.Fatalf("verify after get: %q", out)
	}
	if code, out, e := w.kigumi("run", w.app); code != 0 || strings.TrimSpace(out) != "100" {
		t.Fatalf("run: %d %q %s", code, out, e)
	}
}

func TestModTidyNeverCreatesALock(t *testing.T) {
	w := newWorld(t)
	w.diamond()
	os.Remove(filepath.Join(w.app, "mod.lock.kg"))
	if code, _, e := w.kigumi("mod", "tidy", "-C", w.app); code != 0 {
		t.Fatalf("tidy: %s", e)
	}
	if _, err := os.Stat(filepath.Join(w.app, "mod.lock.kg")); err == nil {
		t.Fatal("tidy must not write a lock where there was none")
	}
}

func localDepWorld(t *testing.T) *world {
	w := newWorld(t)
	w.repo("other", map[string][2]string{"v1.0.0": {"", "1"}, "v1.1.0": {"", "2"}})
	w.repo("dep", map[string][2]string{"v1.0.0": {w.require("other", "v1.0.0"), "100"}})
	w.repo("unused", map[string][2]string{"v1.0.0": {"", "0"}})
	w.write(filepath.Join(w.root, "localdep", "mod.kg"), string(clitest.Manifest("example.com/dep", w.require("other", "v1.1.0"))))
	w.write(filepath.Join(w.root, "localdep", "dep.kg"), "pub fn v() -> Int {\n    999\n}\n")
	w.write(filepath.Join(w.app, "mod.kg"), string(clitest.Manifest("app", w.require("dep", "v1.0.0")+w.require("unused", "v1.0.0"))))
	w.write(filepath.Join(w.app, "main.kg"), "import dep from example.com/dep\nprint \"${dep.v()}\"\n")
	w.write(filepath.Join(w.app, "mod.local.kg"), "Replace {\n    name: \"example.com/dep\"\n    path: \"../localdep\"\n}\n")
	if code, _, e := w.kigumi("get", w.app); code != 0 {
		t.Fatalf("get: %s", e)
	}
	if code, _, e := w.kigumi("get", "--no-local", w.app); code != 0 {
		t.Fatalf("get --no-local: %s", e)
	}
	return w
}

func TestModVerifyHonorsNoLocal(t *testing.T) {
	w := localDepWorld(t)
	if code, _, _ := w.kigumi("mod", "verify", "-C", w.app); code == 0 {
		t.Fatal("setup: with the local replace in effect the lock must disagree")
	}
	if code, out, _ := w.kigumi("mod", "verify", "--no-local", "-C", w.app); code != 0 {
		t.Fatalf("verify --no-local: %q", out)
	}
	t.Setenv("KIGUMI_LOCAL", "off")
	if code, out, _ := w.kigumi("mod", "verify", "-C", w.app); code != 0 {
		t.Fatalf("verify with KIGUMI_LOCAL=off: %q", out)
	}
}

func TestModTidyHonorsNoLocal(t *testing.T) {
	w := localDepWorld(t)
	if code, _, e := w.kigumi("mod", "tidy", "--no-local", "-C", w.app); code != 0 {
		t.Fatalf("tidy --no-local: %s", e)
	}
	lock := w.read("mod.lock.kg")
	if strings.Contains(lock, "example.com/unused") || strings.Count(lock, "version: \"v1.0.0\"") != 2 || strings.Contains(lock, "v1.1.0") {
		t.Fatalf("tidy --no-local must only drop the unused entry:\n%s", lock)
	}
}

func TestModVerifyAfterGetOnAFreshCache(t *testing.T) {
	w := newWorld(t)
	w.repo("foo", map[string][2]string{"v1.0.0": {"", "1"}})
	w.repo("mid", map[string][2]string{"v1.0.0": {w.require("foo", "v1.0.0"), "1"}, "v1.1.0": {"", "2"}})
	w.repo("other", map[string][2]string{"v1.0.0": {w.require("mid", "v1.1.0"), "1"}})
	w.write(filepath.Join(w.app, "mod.kg"), string(clitest.Manifest("app", w.require("mid", "v1.0.0")+w.require("other", "v1.0.0"))))
	w.write(filepath.Join(w.app, "main.kg"), "import mid from example.com/mid\nimport other from example.com/other\nprint \"${mid.v() + other.v()}\"\n")
	if code, _, e := w.kigumi("get", w.app); code != 0 {
		t.Fatalf("get: %s", e)
	}
	if strings.Contains(w.read("mod.lock.kg"), "example.com/foo") {
		t.Fatalf("a module reachable only through a superseded version must not be locked:\n%s", w.read("mod.lock.kg"))
	}
	if code, out, _ := w.kigumi("mod", "verify", "-C", w.app); code != 0 {
		t.Fatalf("verify right after get: %q", out)
	}
}

func TestModVerifyNoticesARequireURLChange(t *testing.T) {
	w := newWorld(t)
	w.repo("dep", map[string][2]string{"v1.0.0": {"", "100"}})
	w.write(filepath.Join(w.app, "mod.kg"), string(clitest.Manifest("app", w.require("dep", "v1.0.0"))))
	w.write(filepath.Join(w.app, "main.kg"), "import dep from example.com/dep\nprint \"${dep.v()}\"\n")
	if code, _, e := w.kigumi("get", w.app); code != 0 {
		t.Fatalf("get: %s", e)
	}
	clitest.Git(t, w.root, "clone", "-q", filepath.Join(w.repos, "dep"), filepath.Join(w.root, "elsewhere", "dep"))
	moved := strings.ReplaceAll(w.read("mod.kg"), w.repos, filepath.Join(w.root, "elsewhere"))
	w.write(filepath.Join(w.app, "mod.kg"), moved)
	code, out, _ := w.kigumi("mod", "verify", "-C", w.app)
	if code != 1 || !strings.Contains(out, "the lock recorded") {
		t.Fatalf("verify: %d %q", code, out)
	}
	if rc, _, _ := w.kigumi("run", w.app); rc == 0 {
		t.Fatal("run is expected to refuse the new source, which is what verify must agree with")
	}
}

func TestRunWarnsAboutAReplaceNothingRequiresAndReleaseRefusesIt(t *testing.T) {
	w := newWorld(t)
	w.repo("dep", map[string][2]string{"v1.0.0": {"", "100"}})
	w.write(filepath.Join(w.app, "mod.kg"), string(clitest.Manifest("app", w.require("dep", "v1.0.0"))))
	w.write(filepath.Join(w.app, "main.kg"), "import dep from example.com/dep\nprint \"${dep.v()}\"\n")
	w.write(filepath.Join(w.root, "loc", "mod.kg"), string(clitest.Manifest("example.com/dep", "")))
	if code, _, e := w.kigumi("get", w.app); code != 0 {
		t.Fatalf("get: %s", e)
	}
	for name, hint := range map[string]string{"example.com/Dep": `did you mean "example.com/dep"`, "example.com/zzz": "names a module nothing in the dependency graph requires"} {
		w.write(filepath.Join(w.app, "mod.local.kg"), "Replace {\n    name: \""+name+"\"\n    path: \"../loc\"\n}\n")
		code, out, e := w.kigumi("run", w.app)
		if code != 0 || strings.TrimSpace(out) != "100" || !strings.Contains(e, "warning[local-replace-unused]: Replace "+name) || !strings.Contains(e, hint) {
			t.Errorf("run with Replace %q: exit %d, stdout %q, stderr %q", name, code, out, e)
		}
		if code, _, e := w.kigumi("check", "--release", w.app); code == 0 || !strings.Contains(e, "release-local-replace") {
			t.Errorf("check --release with Replace %q: exit %d, stderr %q", name, code, e)
		}
	}
}

func TestParseLocalSyntaxErrorStaysAnErrorEverywhere(t *testing.T) {
	w := newWorld(t)
	w.write(filepath.Join(w.app, "mod.kg"), string(clitest.Manifest("app", "")))
	w.write(filepath.Join(w.app, "main.kg"), "print 1\n")
	w.write(filepath.Join(w.app, "mod.local.kg"), "Replace {\n    name: \"Example.com/dep\"\n    path: \"../loc\"\n}\n")
	if code, _, e := w.kigumi("run", w.app); code == 0 || !strings.Contains(e, "not canonical") {
		t.Fatalf("run: exit %d, stderr %q", code, e)
	}
}
