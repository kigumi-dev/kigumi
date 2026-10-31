package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const derivableEqProgram = `
type Point = {
    pub x Int
}

fn useEq(a: Point, b: Point) -> Bool {
    a == b
}

print "${useEq(Point { x: 1 }, Point { x: 1 })}"
`

func loadAnalysis(t *testing.T, main string) *driver.Analysis {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"prov\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(main), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", a.Res.Render())
	}
	return a
}

func TestAnalyzeProvenanceEmptyForPlainProgram(t *testing.T) {
	a := loadAnalysis(t, "print \"hello\"\n")
	if len(a.Provenance) != 0 {
		t.Errorf("Provenance = %+v, want none", a.Provenance)
	}
}

func TestAnalyzeProvenanceRecordsDerivedEq(t *testing.T) {
	a1 := loadAnalysis(t, derivableEqProgram)
	if len(a1.Provenance) == 0 {
		t.Fatal("expected derive provenance for a type using generated Eq")
	}
	var got *struct {
		Generator, Name, Hash string
	}
	for _, p := range a1.Provenance {
		if p.Generator == "derive:Eq" && p.Origin != nil && p.Origin.Name == "Point" {
			got = &struct{ Generator, Name, Hash string }{p.Generator, p.Origin.Name, p.ContentHash}
		}
	}
	if got == nil || got.Hash == "" {
		t.Fatalf("no derive:Eq provenance for Point in %+v", a1.Provenance)
	}
	a2 := loadAnalysis(t, derivableEqProgram)
	var hash2 string
	for _, p := range a2.Provenance {
		if p.Generator == "derive:Eq" && p.Origin != nil && p.Origin.Name == "Point" {
			hash2 = p.ContentHash
		}
	}
	if hash2 != got.Hash {
		t.Errorf("non-deterministic provenance hash: %s vs %s", got.Hash, hash2)
	}
}

func TestAnalyzeProvenanceRecordsSynthesizedType(t *testing.T) {
	a := loadAnalysis(t, "let x: i64? = Some(5)\nprint \"${hash(&x)}\"\n")
	var hash string
	var found bool
	for _, p := range a.Provenance {
		if p.Generator == "derive:Hash" && p.Origin != nil && p.Origin.Name == "Option" {
			found, hash = true, p.ContentHash
		}
	}
	if !found {
		t.Fatalf("no derive:Hash provenance for Option in %+v", a.Provenance)
	}
	if hash == "" {
		t.Error("ContentHash = \"\", want non-empty")
	}
}
