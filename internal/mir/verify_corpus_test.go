package mir_test

import (
	"path/filepath"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/testkit"
)

// TestVerifyCorpus runs the verifier over the MIR of every run fixture,
// std included: a program the builder produces without diagnostics has to
// satisfy the contract, and ask for no conformance sem had not settled before sealing the witness table.
func TestVerifyCorpus(t *testing.T) {
	std := testkit.LoadStd(t, "../../std")
	files, _ := filepath.Glob("../../testdata/run/*.txtar")
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			res, _, _ := checkFixture(t, path, std)
			prog := mir.Build(res)
			if prog.HasErrors() {
				t.Skip("ownership diagnostics")
			}
			if err := prog.Verify(); err != nil {
				t.Errorf("%v", err)
			}
			// Apply verifies again afterwards, so the drops and releases
			// plan-rc makes explicit are checked too.
			if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
				t.Errorf("plan-rc: %v", err)
			}
			mir.Reachable(prog)
			if n := res.ConformMissesAfterSeal(); n != 0 {
				t.Errorf("%d conformance queries first asked after resolveRefCells sealed the witness table", n)
			}
		})
	}
}
