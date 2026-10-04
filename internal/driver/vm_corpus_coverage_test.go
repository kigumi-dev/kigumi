package driver_test

import (
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

// TestRunCorpusVMCoverage checks every testdata/run fixture either runs on the VM or declares why not in `-- engines --`.
func TestRunCorpusVMCoverage(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/run/*.txtar")
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			declared := fixtureEngines(path)
			root, _, _, _ := loadRunFixture(t, path)
			m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			a, err := driver.Analyze(m)
			if err != nil {
				t.Fatal(err)
			}
			if a.Res.HasErrors() {
				t.Fatalf("check errors:\n%s", a.Res.Render())
			}
			ok, why := vm.Supports(a.Prog)
			if ok {
				if declared != "" {
					t.Fatalf("declares %q but the VM actually runs it; drop the -- engines -- section", declared)
				}
				return
			}
			if declared == "" {
				t.Fatalf("the VM refuses this fixture (%s) with no -- engines -- declaring why", why)
			}
		})
	}
}
