package get_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestGetNativeConsentDiff checks that `kigumi get` reports native Link/Source
// changes as a consent summary, and as JSON via --diff-json.
func TestGetNativeConsentDiff(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "greet")
	os.MkdirAll(repo, 0o755)
	os.WriteFile(filepath.Join(repo, "mod.kg"), clitest.Manifest("greet", "Link {\n    library: \"m\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "greet.kg"), []byte("pub fn greeting() -> String {\n    \"hi\"\n}\n"), 0o644)
	clitest.Git(t, repo, "init", "-q")
	clitest.Git(t, repo, "add", ".")
	clitest.Git(t, repo, "commit", "-q", "-m", "initial")
	clitest.Git(t, repo, "tag", "v1.0.0")

	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", "Require {\n    name: \"greet\"\n    version: \"v1.0.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {greeting} from greet\nprint greeting()\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	code, out, errOut := clitest.Execute(t, "get", "--diff-json", app)
	if code != 0 {
		t.Fatalf("get: %s%s", out, errOut)
	}
	if !strings.Contains(errOut, "native changes:") || !strings.Contains(errOut, "greet: new dependency, trusted for the first time") || !strings.Contains(errOut, "+Link library=m") {
		t.Fatalf("missing the first-fetch native consent summary: %s", errOut)
	}
	diff := parseDiff(t, out)
	if len(diff) != 1 || diff[0].Module != "greet" || diff[0].Before != nil || diff[0].After == nil || len(diff[0].After.Links) != 1 || diff[0].After.Links[0].Library != "m" {
		t.Fatalf("JSON diff %+v, want greet adding library m", diff)
	}

	// A second `get` with nothing to fetch reports no native changes, and --diff-json an empty diff.
	code, out, errOut = clitest.Execute(t, "get", "--diff-json", app)
	if code != 0 {
		t.Fatalf("get (no-op): %s%s", out, errOut)
	}
	if diff := parseDiff(t, out); diff == nil || len(diff) != 0 || strings.Contains(errOut, "native changes:") {
		t.Fatalf("an unchanged dependency tree should report an empty diff: stdout %q stderr %q", out, errOut)
	}

	// Bump to a version with a Source: the old v1.0.0 cache entry stays as the diff's "before".
	os.WriteFile(filepath.Join(repo, "mod.kg"), clitest.Manifest("greet", "Link {\n    library: \"m\"\n}\nSource {\n    c: \"native.c\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "native.c"), []byte("int unused;\n"), 0o644)
	clitest.Git(t, repo, "add", ".")
	clitest.Git(t, repo, "commit", "-q", "-am", "add native source")
	clitest.Git(t, repo, "tag", "v1.1.0")
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", "Require {\n    name: \"greet\"\n    version: \"v1.1.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)

	code, out, errOut = clitest.Execute(t, "get", app)
	if code != 0 {
		t.Fatalf("get after version bump: %s%s", out, errOut)
	}
	if !strings.Contains(out, "greet: +Source native.c") {
		t.Fatalf("missing the added-source diff: %s", out)
	}
	if strings.Contains(out, "greet: +Link") {
		t.Fatalf("the unchanged Link must not show as added again: %s", out)
	}
}

type diffEntry struct {
	Module        string
	Before, After *struct{ Links []struct{ Library string } }
}

// parseDiff fails unless stdout is exactly one JSON document.
func parseDiff(t *testing.T, stdout string) []diffEntry {
	t.Helper()
	var diff []diffEntry
	dec := json.NewDecoder(strings.NewReader(stdout))
	if err := dec.Decode(&diff); err != nil || dec.More() {
		t.Fatalf("stdout is not one JSON document (%v): %q", err, stdout)
	}
	return diff
}
