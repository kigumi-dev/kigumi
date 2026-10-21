package driver_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

// TestPlanJSONPerArtifactProvenance covers build --plan deriving one
// artifact's own provenance (its module needs Eq) while build.kg's module (which does not) stays empty, without compiling the artifact.
func TestPlanJSONPerArtifactProvenance(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	write := func(rel, body string) {
		os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
		os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644)
	}
	write("mod.kg", "Module {\n    name: \"planprov\"\n    kigumi: \"0.1\"\n}\n")
	write("cmd/app/main.kg", "type Point = {\n    pub x Int\n}\n\nfn useEq(a: Point, b: Point) -> Bool {\n    a == b\n}\n\nprint \"${useEq(Point { x: 1 }, Point { x: 1 })}\"\n")
	write("build.kg", "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let app = b.executable(\"app\", \"cmd/app\", b.request().target())?\n}\n")
	std, _ := filepath.Abs("../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	g, err := p.Run(driver.HostTarget(), nil, out)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := driver.ResolveGraph(testCompiler(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		DeriveProvenance []struct{ Generator string } `json:"deriveProvenance"`
		Artifacts        []struct {
			Name             string
			DeriveProvenance []struct{ Generator string } `json:"deriveProvenance"`
		}
	}
	if err := json.Unmarshal([]byte(driver.PlanJSON(rg, p)), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.DeriveProvenance) != 0 {
		t.Errorf("build.kg's own provenance = %+v, want none", plan.DeriveProvenance)
	}
	if len(plan.Artifacts) != 1 {
		t.Fatalf("artifacts = %d, want 1", len(plan.Artifacts))
	}
	app := plan.Artifacts[0]
	if len(app.DeriveProvenance) == 0 {
		t.Fatal("expected app's own derive provenance for its Eq usage")
	}
	if app.DeriveProvenance[0].Generator != "derive:Eq" {
		t.Errorf("generator = %q, want derive:Eq", app.DeriveProvenance[0].Generator)
	}
}

// TestPlanJSONPerArtifactProvenanceScoped covers two cmd/* artifacts in one
// module, only one of which derives Eq: each artifact's own provenance must stay scoped to its own entry, never union in a sibling's.
func TestPlanJSONPerArtifactProvenanceScoped(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	write := func(rel, body string) {
		os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
		os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644)
	}
	write("mod.kg", "Module {\n    name: \"planprovscoped\"\n    kigumi: \"0.1\"\n}\n")
	write("cmd/a/main.kg", "type Point = {\n    pub x Int\n}\n\nfn useEq(a: Point, b: Point) -> Bool {\n    a == b\n}\n\nprint \"${useEq(Point { x: 1 }, Point { x: 1 })}\"\n")
	write("cmd/b/main.kg", "print \"no eq here\"\n")
	write("build.kg", "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let a = b.executable(\"a\", \"cmd/a\", b.request().target())?\n    let bb = b.executable(\"b\", \"cmd/b\", b.request().target())?\n}\n")
	std, _ := filepath.Abs("../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	g, err := p.Run(driver.HostTarget(), nil, out)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := driver.ResolveGraph(testCompiler(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Artifacts []struct {
			Name             string
			DeriveProvenance []struct{ Generator string } `json:"deriveProvenance"`
		}
	}
	if err := json.Unmarshal([]byte(driver.PlanJSON(rg, p)), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Artifacts) != 2 {
		t.Fatalf("artifacts = %d, want 2", len(plan.Artifacts))
	}
	byName := map[string][]struct{ Generator string }{}
	for _, art := range plan.Artifacts {
		byName[art.Name] = art.DeriveProvenance
	}
	if len(byName["a"]) == 0 || byName["a"][0].Generator != "derive:Eq" {
		t.Errorf("a's provenance = %+v, want derive:Eq", byName["a"])
	}
	if len(byName["b"]) != 0 {
		t.Errorf("b's provenance = %+v, want none (b does not use Eq)", byName["b"])
	}
}
