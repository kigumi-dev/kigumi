package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// writeScriptTree writes a manifest-less directory and loads one file from it as a script.
func writeScriptTree(t *testing.T, files map[string]string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{
		Entry:      "main.kg",
		ScriptOnly: true,
		Compiler:   testCompilerAt(root),
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A manifest-less script owns only its entry: a sibling's top-level statement and parse error are not package
// diagnostics, and the sibling is not indexed at all.
func TestScriptOnlyIgnoresSiblingScripts(t *testing.T) {
	m := writeScriptTree(t, map[string]string{
		"main.kg":    "print \"entry\"\n",
		"other.kg":   "pub fn helper() -> Int { 1 }\nprint \"sibling\"\n",
		"broken.kg":  "fn broken( {\n",
		"helper.kg":  "fn helper() -> Int { 1 }\n",
		"ignored.md": "not a source\n",
	})
	own := 0
	for _, path := range m.Order {
		if p := m.Packages[path]; !p.Std && p.Module == "" {
			own++
		}
	}
	if own != 1 {
		t.Fatalf("own packages = %d (%v), want only the entry's package", own, m.Order)
	}
	pkg, ok := m.Packages["main"]
	if !ok {
		t.Fatalf("Packages = %v, want the entry package main", m.Order)
	}
	if len(pkg.Files) != 1 || pkg.Files[0].File.Name != "main.kg" {
		t.Fatalf("package files = %v, want only main.kg", pkg.Files)
	}
	res, err := driver.CheckOnly(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("a sibling must not add diagnostics:\n%s", res.Render())
	}
	if strings.Contains(res.Render(), "E915") {
		t.Fatalf("a sibling's top-level statement must not be E915:\n%s", res.Render())
	}
}

// A name declared only in a sibling is not visible from the script, so lookup reports E100 instead of finding it.
func TestScriptOnlySiblingNameIsNotVisible(t *testing.T) {
	m := writeScriptTree(t, map[string]string{
		"main.kg":  "print \"${helper()}\"\n",
		"other.kg": "fn helper() -> Int { 1 }\n",
	})
	res, err := driver.CheckOnly(m)
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E100") || !strings.Contains(out, "helper") {
		t.Fatalf("want E100 for the sibling-only name, got:\n%s", out)
	}
}

// The script-only rule is manifest-less only: a mod.kg module still compiles its directory as one package even
// when a caller asks for ScriptOnly, so a non-entry sibling's top-level statement is E915 as before.
func TestModuleStillCompilesSiblings(t *testing.T) {
	m := moduleOf(t, "sibling", map[string]string{
		"main.kg":    "print 1\n",
		"other.kg":   "let x = 2\n",
		"sub/sub.kg": "pub fn n() -> Int { 1 }\n",
	}, driver.LoadOptions{ScriptOnly: true})
	res, err := driver.CheckOnly(m)
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if !strings.Contains(out, "E915") || !strings.Contains(out, "other.kg") {
		t.Fatalf("a module must report its sibling's top-level statement as E915, got:\n%s", out)
	}
}
