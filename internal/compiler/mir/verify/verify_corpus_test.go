package verify_test

import (
	"kigumi/internal/devkit/fixture"
	"path/filepath"
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
	mirreach "kigumi/internal/compiler/mir/reach"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
	"kigumi/internal/devkit/semfixture"
)

// TestVerifyCorpus runs the verifier over the MIR of every run fixture,
// std included: a program the builder produces without diagnostics has to
// satisfy the contract, and ask for no conformance sem had not settled before sealing the witness table.
func TestVerifyCorpus(t *testing.T) {
	files, _ := filepath.Glob("../../../../testdata/run/*.txtar")
	for _, path := range files {
		if !fixture.Shard(filepath.Base(path)) {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			res, _, _ := mirfixture.CheckFixture(t, path, semfixture.LoadStd(t, "../../../../std"))
			prog := mirbuild.Program(res)
			if prog.HasErrors() {
				t.Skip("ownership diagnostics")
			}
			if err := mirverify.Program(prog); err != nil {
				t.Errorf("%v", err)
			}
			// Apply verifies again afterwards, so the drops and releases
			// plan-rc makes explicit are checked too.
			if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
				t.Errorf("plan-rc: %v", err)
			}
			if err := mirplan.Apply(prog, mirplan.Transform{Name: "elide-rc", Run: mirplan.ElideRC}); err != nil {
				t.Errorf("elide-rc: %v", err)
			}
			if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect}); err != nil {
				t.Errorf("plan-direct: %v", err)
			}
			mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)
			if n := res.ConformMissesAfterSeal(); n != 0 {
				t.Errorf("%d conformance queries first asked after resolveRefCells sealed the witness table", n)
			}
		})
	}
}
