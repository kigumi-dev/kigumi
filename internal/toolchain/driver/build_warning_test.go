package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A successful build reports the same semantic warnings as check.
func TestBuildReportsSemanticWarnings(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte("import fmt from std/fmt\n\nprint \"ok\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ok, err := driver.BuildWith(m, filepath.Join(t.TempDir(), "prog"), &stderr, driver.BuildOptions{})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning[W100]") || !strings.Contains(stderr.String(), "unused import") {
		t.Fatalf("build dropped the semantic warning:\n%s", stderr.String())
	}
}
