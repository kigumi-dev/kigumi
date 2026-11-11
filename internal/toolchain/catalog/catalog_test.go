package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/catalog"
)

const stdRoot = "../../../std"

// TestGenerateStdCatalog proves the committed policy completes into a catalog
// covering every std package with the resolver's layer.
func TestGenerateStdCatalog(t *testing.T) {
	c, err := catalog.Generate(stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	if c.Format != catalog.Format || c.FormatVersion != catalog.FormatVersion {
		t.Fatalf("format = %q/%d", c.Format, c.FormatVersion)
	}
	byPath := map[string]catalog.Entry{}
	for _, e := range c.Packages {
		byPath[e.Path] = e
	}
	for _, path := range sem.StdLayerPaths() {
		e, ok := byPath[path]
		if !ok {
			t.Errorf("%s missing from the catalog", path)
			continue
		}
		if e.Layer == catalog.LayerNone {
			t.Errorf("%s has no layer", path)
		}
		if e.Visibility == catalog.VisibilityInternal && e.Audience == catalog.AudienceUser {
			t.Errorf("%s is internal but aims at users", path)
		}
	}
	if got, want := len(c.Packages), len(sem.StdLayerPaths())+9; got != want {
		t.Errorf("packages = %d, want %d (std + exp rows)", got, want)
	}
	if e := byPath["std/build"]; e.Audience != catalog.AudienceTooling {
		t.Errorf("std/build audience = %s, want tooling", e.Audience)
	}
	if e := byPath["std/shell/coreutil"]; e.Visibility != catalog.VisibilityInternal {
		t.Errorf("std/shell/coreutil visibility = %s, want internal", e.Visibility)
	}
	for _, path := range []string{"std/json", "std/os", "std/task", "std/error"} {
		if len(byPath[path].SPI) == 0 {
			t.Errorf("%s exposes no doc-marked SPI", path)
		}
	}
	if e := byPath["exp/ansi"]; e.Layer != catalog.LayerNone || e.Support != catalog.SupportUnverified {
		t.Errorf("exp/ansi = %+v, want a layer-less unverified row", e)
	}
	if e := byPath["std/json"]; e.Declarations.Functions == 0 || e.Declarations.Types == 0 {
		t.Errorf("std/json declaration counts not generated: %+v", e.Declarations)
	}
}

// TestValidateRejectsStaleCatalog catches a catalog that no longer matches the
// sources: a wrong layer, a wrong SPI name and a missing package.
func TestValidateRejectsStaleCatalog(t *testing.T) {
	c, err := catalog.Generate(stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	mutated := *c
	mutated.Packages = append([]catalog.Entry(nil), c.Packages...)
	for i := range mutated.Packages {
		switch mutated.Packages[i].Path {
		case "std/io":
			mutated.Packages[i].Layer = catalog.LayerFoundation
		case "std/json":
			mutated.Packages[i].SPI = append(mutated.Packages[i].SPI, catalog.SPI{Name: "Fields.missing"})
		}
	}
	var kept []catalog.Entry
	for _, e := range mutated.Packages {
		if e.Path != "std/map" {
			kept = append(kept, e)
		}
	}
	mutated.Packages = kept
	err = catalog.Validate(stdRoot, &mutated)
	if err == nil {
		t.Fatal("Validate accepted a stale catalog")
	}
	for _, want := range []string{"std/io", "layer", "Fields.missing", "std/map"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%s", want, err)
		}
	}
}

// TestGenerateRejectsMarkersOnPrivateDeclarations keeps SPI from being claimed
// for a declaration user code cannot even see.
func TestGenerateRejectsMarkersOnPrivateDeclarations(t *testing.T) {
	root := writeTree(t, "std/x", `/// Internal SPI: generated code calls this.
fn hidden() -> Int {
    1
}
`)
	if _, err := catalog.Generate(root); err == nil || !strings.Contains(err.Error(), "not pub") {
		t.Fatalf("Generate = %v, want a not-pub error", err)
	}
}

// TestGenerateRejectsUncoveredPackage checks both coverage directions.
func TestGenerateRejectsUncoveredPackage(t *testing.T) {
	root := writeTree(t, "std/x", "pub fn f() -> Int {\n    1\n}\n")
	if err := os.WriteFile(filepath.Join(root, "catalog", "catalog.json"), []byte(`{"format": "kigumi.catalog", "formatVersion": 1, "packages": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Generate(root); err == nil || !strings.Contains(err.Error(), "std/x has no catalog entry") {
		t.Fatalf("Generate = %v, want a missing-entry error", err)
	}
	root = writeTree(t, "std/x", "pub fn f() -> Int {\n    1\n}\n")
	if _, err := catalog.Generate(root); err == nil || !strings.Contains(err.Error(), "not in the std layer table") {
		t.Fatalf("Generate = %v, want a layer-table error", err)
	}
}

// writeTree writes one std package plus a policy row for it. The policy row's
// layer is foundation, which is wrong for a hosted-only exp row but matches
// the shape Generate parses before it looks at the resolver.
func writeTree(t *testing.T, pkg, src string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, strings.TrimPrefix(pkg, "std/"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := `{
  "format": "kigumi.catalog",
  "formatVersion": 1,
  "packages": [
    {"path": "` + pkg + `", "layer": "foundation", "stability": "stable", "audience": "user", "support": "supported", "visibility": "public", "availability": {"sys": ["freestanding", "none", "hosted"], "targets": ["all"]}}
  ]
}
`
	if err := os.MkdirAll(filepath.Join(root, "catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "catalog", "catalog.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
