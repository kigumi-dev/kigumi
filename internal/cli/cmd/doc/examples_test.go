package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// examples/catalog.json is the file the example runner itself checks against.
func TestDocExamples(t *testing.T) {
	root, _ := filepath.Abs("../../../../examples")
	code, out, errOut := clitest.Execute(t, "doc", "examples", root)
	if code != 0 {
		t.Fatalf("doc examples: exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"# Examples", "### hello", "### repostat"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDocExamplesHTML(t *testing.T) {
	root, _ := filepath.Abs("../../../../examples")
	outDir := filepath.Join(t.TempDir(), "site")
	code, _, errOut := clitest.Execute(t, "doc", "examples", "--html", "-o", outDir, root)
	if code != 0 {
		t.Fatalf("doc examples --html: exit %d\n%s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(outDir, "examples.html")); err != nil {
		t.Errorf("examples.html: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Errorf("index.html: %v", err)
	}
}
