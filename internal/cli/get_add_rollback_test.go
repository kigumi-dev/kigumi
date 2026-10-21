package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetAddRollsBackOnFetchFailure checks that get <module> removes the Require it appended before fetching if the fetch then fails.
func TestGetAddRollsBackOnFetchFailure(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), manifest("app", ""), 0o644)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// A URL that cannot be cloned, so the fetch step fails deterministically.
	notARepo := filepath.Join(root, "does-not-exist")
	if code, _, errOut := execute(t, "get", "example.com/nope", "--url", "file://"+notARepo, app); code == 0 {
		t.Fatalf("fetch of a nonexistent repo must fail, stderr %q", errOut)
	}
	mf, err := os.ReadFile(filepath.Join(app, "mod.kg"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mf), "example.com/nope") {
		t.Fatalf("a Require that failed to fetch must not linger in the manifest, got:\n%s", mf)
	}
}
