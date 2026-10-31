package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func TestSelectVersionsRefusesUrlChangeAtHigherVersion(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	putManifest(t, "x", "v1.0.0", "")
	putManifest(t, "mid", "v1.0.0", "Require { name: \"x\", version: \"v1.5.0\", url: \"https://evil.example/x.git\" }\n")
	root := driver.ModFile{Requires: []driver.Require{
		{Name: "x", Version: "v1.0.0", URL: "https://good.example/x.git"},
		{Name: "mid", Version: "v1.0.0", URL: "https://good.example/mid.git"},
	}}
	_, err := driver.SelectVersions(testCompiler(), root, driver.Local{})
	if err == nil || !strings.Contains(err.Error(), "https://good.example/x") || !strings.Contains(err.Error(), "https://evil.example/x") {
		t.Fatalf("want an error naming both urls, got %v", err)
	}
}

func TestSelectVersionsRefusesUrlOverDerivedRootSource(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	putManifest(t, "github.com/acme/x", "v1.0.0", "")
	putManifest(t, "mid", "v1.0.0", "Require { name: \"github.com/acme/x\", version: \"v1.5.0\", url: \"https://evil.example/x.git\" }\n")
	root := driver.ModFile{Requires: []driver.Require{
		{Name: "github.com/acme/x", Version: "v1.0.0"},
		{Name: "mid", Version: "v1.0.0", URL: "https://good.example/mid.git"},
	}}
	_, err := driver.SelectVersions(testCompiler(), root, driver.Local{})
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
	root := driver.ModFile{Requires: []driver.Require{
		{Name: "x", Version: "v1.0.0", URL: "https://good.example/x.git"},
		{Name: "mid", Version: "v1.0.0", URL: "https://good.example/mid.git"},
		{Name: "other", Version: "v1.0.0", URL: "https://good.example/other.git"},
	}}
	sel, err := driver.SelectVersions(testCompiler(), root, driver.Local{})
	if err != nil {
		t.Fatal(err)
	}
	if got := sel.Requires["x"]; got.Version != "v1.5.0" || driver.RepoURLKey(got) != "https://good.example/x" {
		t.Fatalf("selected %+v", got)
	}
}

func TestSelectVersionsTransitiveUrlsMustAgree(t *testing.T) {
	for _, order := range [][2]string{{"a", "b"}, {"b", "a"}} {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		putManifest(t, "a", "v1.0.0", "Require { name: \"x\", version: \"v1.0.0\", url: \"https://one.example/x.git\" }\n")
		putManifest(t, "b", "v1.0.0", "Require { name: \"x\", version: \"v1.0.0\", url: \"https://two.example/x.git\" }\n")
		root := driver.ModFile{Requires: []driver.Require{
			{Name: order[0], Version: "v1.0.0", URL: "https://h.example/" + order[0] + ".git"},
			{Name: order[1], Version: "v1.0.0", URL: "https://h.example/" + order[1] + ".git"},
		}}
		_, err := driver.SelectVersions(testCompiler(), root, driver.Local{})
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
		root := driver.ModFile{Requires: []driver.Require{
			{Name: order[0], Version: "v1.0.0", URL: "https://h.example/" + order[0] + ".git"},
			{Name: order[1], Version: "v1.0.0", URL: "https://h.example/" + order[1] + ".git"},
		}}
		_, err := driver.SelectVersions(testCompiler(), root, driver.Local{})
		if err == nil || !strings.Contains(err.Error(), "https://github.com/acme/x") || !strings.Contains(err.Error(), "https://evil.example/x") {
			t.Fatalf("order %v: want an error naming both urls, got %v", order, err)
		}
	}
}
