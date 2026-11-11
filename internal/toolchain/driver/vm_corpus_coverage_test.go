package driver_test

import (
	"path/filepath"
	"testing"

	devfixture "kigumi/internal/devkit/fixture"
)

// TestRunCorpusVMCoverage checks every testdata/run fixture either runs on the VM or declares why not in `-- engines --`.
func TestRunCorpusVMCoverage(t *testing.T) {
	files, err := devfixture.Corpus("../../..", "run")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	for _, path := range devfixture.ShardPaths(files) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			declared := fixtureEngines(path)
			c := checkedFixture(t, path)
			if c.errs != "" {
				t.Fatalf("check errors:\n%s", c.errs)
			}
			if c.refusal == nil {
				if declared != "" {
					t.Fatalf("declares %q but the VM actually runs it; drop the -- engines -- section", declared)
				}
				return
			}
			if declared == "" {
				t.Fatalf("the VM refuses this fixture (%v) with no -- engines -- declaring why", c.refusal)
			}
		})
	}
}
