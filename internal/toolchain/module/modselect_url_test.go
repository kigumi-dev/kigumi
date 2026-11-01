package module_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestSelectVersionsRefusesUrlChangeAtHigherVersion(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	putManifest(t, "x", "v1.0.0", "")
	putManifest(t, "mid", "v1.0.0", "Require { name: \"x\", version: \"v1.5.0\", url: \"https://evil.example/x.git\" }\n")
	root := module.ModFile{Requires: []module.Require{
		{Name: "x", Version: "v1.0.0", URL: "https://good.example/x.git"},
		{Name: "mid", Version: "v1.0.0", URL: "https://good.example/mid.git"},
	}}
	_, err := module.SelectVersions(testCompiler(), root, module.Local{})
	if err == nil || !strings.Contains(err.Error(), "https://good.example/x") || !strings.Contains(err.Error(), "https://evil.example/x") {
		t.Fatalf("want an error naming both urls, got %v", err)
	}
}

func TestSelectVersionsRefusesUrlOverDerivedRootSource(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	putManifest(t, "github.com/acme/x", "v1.0.0", "")
	putManifest(t, "mid", "v1.0.0", "Require { name: \"github.com/acme/x\", version: \"v1.5.0\", url: \"https://evil.example/x.git\" }\n")
	root := module.ModFile{Requires: []module.Require{
		{Name: "github.com/acme/x", Version: "v1.0.0"},
		{Name: "mid", Version: "v1.0.0", URL: "https://good.example/mid.git"},
	}}
	_, err := module.SelectVersions(testCompiler(), root, module.Local{})
	if err == nil || !strings.Contains(err.Error(), "https://github.com/acme/x") || !strings.Contains(err.Error(), "https://evil.example/x") {
		t.Fatalf("want an error naming both urls, got %v", err)
	}
}

func TestSelectVersionsKeepsRootUrlAndAcceptsSpellingVariants(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for _, v := range []string{"v1.0.0", "v1.2.0", "v1.5.0"} {
		putManifest(t, "x", v, "")
	}
	putManifest(t, "mid", "v1.0.0", "Require { name: \"x\", version: \"v1.5.0\" }\n")
	putManifest(t, "other", "v1.0.0", "Require { name: \"x\", version: \"v1.2.0\", url: \"https://good.example/x/\" }\n")
	root := module.ModFile{Requires: []module.Require{
		{Name: "x", Version: "v1.0.0", URL: "https://good.example/x.git"},
		{Name: "mid", Version: "v1.0.0", URL: "https://good.example/mid.git"},
		{Name: "other", Version: "v1.0.0", URL: "https://good.example/other.git"},
	}}
	sel, err := module.SelectVersions(testCompiler(), root, module.Local{})
	if err != nil {
		t.Fatal(err)
	}
	if got := sel.Requires["x"]; got.Version != "v1.5.0" || module.RepoURLKey(got) != "https://good.example/x" {
		t.Fatalf("selected %+v", got)
	}
}

func TestSelectVersionsTransitiveUrlsMustAgree(t *testing.T) {
	for _, order := range [][2]string{{"a", "b"}, {"b", "a"}} {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		putManifest(t, "a", "v1.0.0", "Require { name: \"x\", version: \"v1.0.0\", url: \"https://one.example/x.git\" }\n")
		putManifest(t, "b", "v1.0.0", "Require { name: \"x\", version: \"v1.0.0\", url: \"https://two.example/x.git\" }\n")
		root := module.ModFile{Requires: []module.Require{
			{Name: order[0], Version: "v1.0.0", URL: "https://h.example/" + order[0] + ".git"},
			{Name: order[1], Version: "v1.0.0", URL: "https://h.example/" + order[1] + ".git"},
		}}
		_, err := module.SelectVersions(testCompiler(), root, module.Local{})
		if err == nil || !strings.Contains(err.Error(), "https://one.example/x") || !strings.Contains(err.Error(), "https://two.example/x") {
			t.Fatalf("order %v: want an error naming both urls, got %v", order, err)
		}
	}
}

func TestSelectVersionsDerivedAndExplicitTransitiveUrlsMustAgree(t *testing.T) {
	for _, order := range [][2]string{{"a", "b"}, {"b", "a"}} {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		putManifest(t, "github.com/acme/x", "v1.0.0", "")
		putManifest(t, "a", "v1.0.0", "Require { name: \"github.com/acme/x\", version: \"v1.0.0\" }\n")
		putManifest(t, "b", "v1.0.0", "Require { name: \"github.com/acme/x\", version: \"v1.0.0\", url: \"https://evil.example/x.git\" }\n")
		root := module.ModFile{Requires: []module.Require{
			{Name: order[0], Version: "v1.0.0", URL: "https://h.example/" + order[0] + ".git"},
			{Name: order[1], Version: "v1.0.0", URL: "https://h.example/" + order[1] + ".git"},
		}}
		_, err := module.SelectVersions(testCompiler(), root, module.Local{})
		if err == nil || !strings.Contains(err.Error(), "https://github.com/acme/x") || !strings.Contains(err.Error(), "https://evil.example/x") {
			t.Fatalf("order %v: want an error naming both urls, got %v", order, err)
		}
	}
}

// TestSelectVersionsOriginConflict covers MOD-005: the same name and
// version from two different origins is a conflict, not a silent merge.
func TestSelectVersionsOriginConflict(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	dir, _ := module.CacheDir(testCompiler(), module.Require{Name: "foo", Version: "v1.0.0"})
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "mod.kg"), manifest("foo", ""), 0o644)
	os.WriteFile(filepath.Join(dir, module.OriginFileName), []byte("cafebabe\n"), 0o644)

	conflicting := module.ModFile{Requires: []module.Require{
		{Name: "foo", Version: "v1.0.0", URL: "https://origin-a.example/foo.git"},
		{Name: "foo", Version: "v1.0.0", URL: "https://origin-b.example/foo.git"},
	}}
	if _, err := module.SelectVersions(testCompiler(), conflicting, module.Local{}); err == nil || !strings.Contains(err.Error(), "origin-a.example") || !strings.Contains(err.Error(), "origin-b.example") {
		t.Fatalf("same name+version from different origins should conflict: %v", err)
	}

	same := module.ModFile{Requires: []module.Require{
		{Name: "foo", Version: "v1.0.0", URL: "https://origin-a.example/foo.git"},
		{Name: "foo", Version: "v1.0.0", URL: "https://origin-a.example/foo.git"},
	}}
	sel, err := module.SelectVersions(testCompiler(), same, module.Local{})
	if err != nil || sel.Requires["foo"].URL != "https://origin-a.example/foo.git" {
		t.Fatalf("identical origins should not conflict: %+v %v", sel, err)
	}
}
