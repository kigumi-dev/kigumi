package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
)

func TestFoldComptimeRefusesAnUnplannedBlock(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"order\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("type Point = {\n    pub mut x Int\n    pub y Int\n}\n\nlet n = comptime {\n    let mut p = Point { x: 1, y: 2 }\n    let mut ps: Array[Point] = Array.empty()\n    ps.push(p)\n    p.x = 9\n    ps[0].x\n}\nprint \"${n}\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := LoadModule(root, LoadOptions{Compiler: internalTestCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	mod, err := m.semModule()
	if err != nil {
		t.Fatal(err)
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	rc := mir.Transform{Name: "plan-rc", Run: mir.PlanRC}
	fold := mir.Transform{Name: "fold-comptime", Run: foldComptime}
	err = mir.Build(res).Apply(rc, fold)
	if err == nil || !strings.Contains(err.Error(), "before plan-direct") {
		t.Fatalf("folding an unplanned block: err = %v", err)
	}
	if err := mir.Build(res).Apply(rc, mir.Transform{Name: "plan-direct", Run: mir.PlanDirect}, fold); err != nil {
		t.Fatalf("folding a planned block: %v", err)
	}
}
