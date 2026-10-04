package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
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
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"tutorial","oracle":"run-output","backends":["vm","native"],"shows":"says hi"}]}`)
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
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"tutorial","oracle":"run-output","backends":["vm","native"],"shows":"says hi"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error for the undeclared mystery example")
	}
}

func TestLoadExampleCatalogRejectsEntryWithoutDirectory(t *testing.T) {
	root := t.TempDir()
	writeCatalog(t, root, `{"examples":[{"name":"ghost","kind":"tutorial","oracle":"run-output","backends":["vm","native"],"shows":"x"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error for a catalog entry with no matching directory")
	}
}

func TestLoadExampleCatalogRejectsMissingReason(t *testing.T) {
	root := t.TempDir()
	writeModDir(t, root, "hello")
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"tutorial","oracle":"test-only","backends":["vm"],"shows":"x"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error: a non-run-output oracle needs a reason")
	}
}

// TestExamplesReadmeMatchesCatalog checks that examples/README.md and examples/catalog.json name the same examples.
func TestExamplesReadmeMatchesCatalog(t *testing.T) {
	root, _ := filepath.Abs("../../examples")
	_, names, err := driver.LoadExampleCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !strings.Contains(string(readme), "["+name+"]("+name+"/)") {
			t.Errorf("examples/README.md has no table row for %q", name)
		}
	}
}

func TestLoadExampleCatalogRejectsUnknownKind(t *testing.T) {
	root := t.TempDir()
	writeModDir(t, root, "hello")
	writeCatalog(t, root, `{"examples":[{"name":"hello","kind":"mystery","oracle":"run-output","backends":["vm"],"shows":"x"}]}`)
	if _, _, err := driver.LoadExampleCatalog(root); err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
}
