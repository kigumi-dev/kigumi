package driver_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/testtool"
)

// Expected stderr policy comes from testdata/manifest.json; a fixture the
// manifest excludes from "vm" is left to TestAotCorpus instead.
func TestRunCorpusEngines(t *testing.T) {
	manifest := loadManifest(t)
	files, _ := filepath.Glob("../../testdata/run/*.txtar")
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	want, ran := 0, 0
	for _, path := range files {
		id := fixtureID(path)
		fx := manifest.ByID(id)
		if fx == nil {
			t.Fatalf("%s: no manifest entry (run UPDATE=1 go test -run TestManifestFresh ./internal/driver/)", id)
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			if !fx.Runs("vm") {
				t.Skipf("manifest: %s only runs native (%v)", id, fx.Skip)
			}
			want++
			root, wantOut, wantErr, wantExit := loadRunFixture(t, path)
			m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"main/main.kg", "one", "two"}
			t.Chdir(t.TempDir())
			var out, errOut bytes.Buffer
			code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, args)
			if err != nil {
				t.Fatalf("manifest says vm runs %s, but it refused: %v", id, err)
			}
			ran++
			if out.String() != wantOut {
				t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", out.String(), wantOut)
			}
			if err := testtool.CheckStderr(fx.Stderr, wantErr, errOut.String()); err != nil {
				t.Error(err)
			}
			if code != wantExit {
				t.Errorf("exit %d, want %d\nstderr:\n%s", code, wantExit, errOut.String())
			}
		})
	}
	t.Logf("vm: expect %d, ran %d", want, ran)
}
