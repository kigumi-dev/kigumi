package driver

import (
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/contract/schema"
)

// A user build lowers only the std bodies it reaches, so std's own MIR-level checks run here over every body.
func TestStdLowersAndVerifiesWhole(t *testing.T) {
	res := sem.CheckWith(&sem.Module{Packages: testkit.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog, err := buildProgram(res, progOptions{verifyEach: true, lowerAll: true})
	if err != nil {
		t.Fatal(err)
	}
	lowered := 0
	for id := 1; id < len(res.Entities); id++ {
		if prog.Unlowered(sem.EntityID(id)) {
			t.Errorf("%s was left unlowered", res.Entity(sem.EntityID(id)).Name)
		}
		if prog.ByEnt[sem.EntityID(id)] != nil && res.Entity(sem.EntityID(id)).Flags&sem.EfStd != 0 {
			lowered++
		}
	}
	if lowered < 500 {
		t.Fatalf("only %d std bodies lowered", lowered)
	}
	for i := 1; i < len(res.Files); i++ {
		if ds := res.Diagnostics(res.Files[i].Tree); len(ds) > 0 {
			t.Errorf("std reports diagnostics:\n%s", diag.RenderAll(res, ds))
		}
	}
	if pruned := mirbuild.Program(res); len(pruned.Funcs) >= len(prog.Funcs) {
		t.Errorf("Build lowered %d bodies of a std-only module, BuildAll %d", len(pruned.Funcs), len(prog.Funcs))
	}
}
