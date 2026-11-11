package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func writeCatalog(t *testing.T, root, json string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeModDir(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("name \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadExampleCatalog(t *testing.T) {
	root := t.TempDir()
	writeModDir(t, root, "hello")
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"tutorial","order":1,"level":"beginner","oracle":"run-output","backends":["vm","native"],"features":["lang.entry"],"shows":"says hi"}]}`)
	catalog, names, err := driver.LoadExampleCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "hello" || catalog["hello"].Shows != "says hi" {
		t.Errorf("catalog: %+v names: %v", catalog, names)
	}
}

// TestLoadExampleCatalogRejectsUndeclaredExample checks that an undeclared example directory fails loudly instead of passing silently.
func TestLoadExampleCatalogRejectsUndeclaredExample(t *testing.T) {
	root := t.TempDir()
	writeModDir(t, root, "hello")
	writeModDir(t, root, "mystery")
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"tutorial","order":1,"level":"beginner","oracle":"run-output","backends":["vm","native"],"features":["lang.entry"],"shows":"says hi"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error for the undeclared mystery example")
	}
}

func TestLoadExampleCatalogRejectsEntryWithoutDirectory(t *testing.T) {
	root := t.TempDir()
	writeCatalog(t, root, `{"examples":[{"name":"ghost","kind":"tutorial","order":1,"level":"beginner","oracle":"run-output","backends":["vm","native"],"features":["lang.entry"],"shows":"x"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error for a catalog entry with no matching directory")
	}
}

func TestLoadExampleCatalogRejectsMissingReason(t *testing.T) {
	root := t.TempDir()
	writeModDir(t, root, "hello")
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"tutorial","order":1,"level":"beginner","oracle":"test-only","backends":["vm"],"features":["lang.entry"],"shows":"x"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error: a non-run-output oracle needs a reason")
	}
}

// TestExamplesReadmeMatchesCatalog checks that the examples/README.md table is
// exactly the catalog rendered in reading order: no missing, extra or stale
// row, and the Covers column matches the catalog's features.
func TestExamplesReadmeMatchesCatalog(t *testing.T) {
	root, _ := filepath.Abs("../../../examples")
	catalog, names, err := driver.LoadExampleCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := driver.RenderExamplesTable(catalog, names)
	if !strings.Contains(string(readme), want) {
		t.Errorf("examples/README.md table does not match the catalog; want:\n%s\n--- readme ---\n%s", want, readme)
	}
	rows := 0
	for _, line := range strings.Split(string(readme), "\n") {
		if strings.HasPrefix(line, "| [") {
			rows++
		}
	}
	if rows != len(names) {
		t.Errorf("examples/README.md has %d example rows, catalog has %d", rows, len(names))
	}
}

func TestLoadExampleCatalogRejectsUnknownKind(t *testing.T) {
	root := t.TempDir()
	writeModDir(t, root, "hello")
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"mystery","order":1,"level":"beginner","oracle":"run-output","backends":["vm"],"features":["lang.entry"],"shows":"x"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
}
