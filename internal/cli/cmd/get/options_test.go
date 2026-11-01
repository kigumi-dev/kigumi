package get_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// A mistyped .kg script argument to `get` must be reported as a missing
// file, not silently reclassified as a module path to add.
func TestGetMissingScriptArg(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing_script.kg")
	if code, errOut := clitest.ExecuteReal(t, "get", missing); code != 2 || !strings.Contains(errOut, "no such file") {
		t.Fatalf("get with no mod.kg: exit %d, stderr %q", code, errOut)
	}
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"example\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	typo := filepath.Join(dir, "typo_script.kg")
	if code, errOut := clitest.ExecuteReal(t, "get", typo); code != 2 || !strings.Contains(errOut, "no such file") {
		t.Fatalf("get with a mod.kg present: exit %d, stderr %q", code, errOut)
	}
}
