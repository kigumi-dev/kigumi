package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A package named "README" must keep README.md; the generated list page moves aside instead.
func TestDocMarkdownIndexNameCollision(t *testing.T) {
	mod := t.TempDir()
	os.MkdirAll(filepath.Join(mod, "README"), 0o755)
	os.WriteFile(filepath.Join(mod, "README", "readme.kg"), []byte("/// README marks the package itself.\npub fn load() -> Int {\n    1\n}\n"), 0o644)
	os.MkdirAll(filepath.Join(mod, "other"), 0o755)
	os.WriteFile(filepath.Join(mod, "other", "other.kg"), []byte("pub fn noop() {}\n"), 0o644)

	outDir := t.TempDir()
	if code, _, errOut := execute(t, "doc", "-o", outDir, mod); code != 0 {
		t.Fatalf("doc -o: exit %d\n%s", code, errOut)
	}

	readme, err := os.ReadFile(filepath.Join(outDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "README marks the package itself.") {
		t.Errorf("README.md should be the README package's own page, got:\n%s", readme)
	}

	list, err := os.ReadFile(filepath.Join(outDir, "README-2.md"))
	if err != nil {
		t.Fatalf("expected a fallback list page README-2.md: %v", err)
	}
	if !strings.Contains(string(list), "[other](other.md)") || !strings.Contains(string(list), "[README](README.md)") {
		t.Errorf("fallback list page missing package links:\n%s", list)
	}
}

// Same collision in HTML mode, with index.html instead of README.md.
func TestDocHTMLIndexNameCollision(t *testing.T) {
	mod := t.TempDir()
	os.MkdirAll(filepath.Join(mod, "index"), 0o755)
	os.WriteFile(filepath.Join(mod, "index", "index.kg"), []byte("/// index marks the package itself.\npub fn load() -> Int {\n    1\n}\n"), 0o644)
	os.MkdirAll(filepath.Join(mod, "other"), 0o755)
	os.WriteFile(filepath.Join(mod, "other", "other.kg"), []byte("pub fn noop() {}\n"), 0o644)

	outDir := t.TempDir()
	if code, _, errOut := execute(t, "doc", "--html", "-o", outDir, mod); code != 0 {
		t.Fatalf("doc --html -o: exit %d\n%s", code, errOut)
	}

	index, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "index marks the package itself.") {
		t.Errorf("index.html should be the index package's own page, got:\n%s", index)
	}

	if _, err := os.Stat(filepath.Join(outDir, "index-2.html")); err != nil {
		t.Errorf("expected a fallback nav page index-2.html: %v", err)
	}
}
