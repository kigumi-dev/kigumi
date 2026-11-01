package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

func TestFoldComptimeRefusesAnUnplannedBlock(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"order\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("type Point = {\n    pub mut x Int\n    pub y Int\n}\n\nlet n = comptime {\n    let mut p = Point { x: 1, y: 2 }\n    let mut ps: Array[Point] = Array.empty()\n    ps.push(p)\n    p.x = 9\n    ps[0].x\n}\nprint \"${n}\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
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
	rc := mirplan.Transform{Name: "plan-rc", Run: mirplan.PlanRC}
	fold := mirplan.Transform{Name: "fold-comptime", Run: foldComptime}
	err = mirplan.Apply(mirbuild.Program(res), rc, fold)
	if err == nil || !strings.Contains(err.Error(), "before plan-direct") {
		t.Fatalf("folding an unplanned block: err = %v", err)
	}
	if err := mirplan.Apply(mirbuild.Program(res), rc, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect}, fold); err != nil {
		t.Fatalf("folding a planned block: %v", err)
	}
}
