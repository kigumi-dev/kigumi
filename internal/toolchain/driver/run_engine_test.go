package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"kigumi/internal/devkit/fixture"
	"kigumi/internal/toolchain/driver"
)

// Expected stderr policy comes from testdata/manifest.json; a fixture the
// manifest excludes from "vm" is left to TestAotCorpus instead.
func TestRunCorpusEngines(t *testing.T) {
	manifest := loadManifest(t)
	files, _ := filepath.Glob("../../../testdata/run/*.txtar")
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	var want, ran atomic.Int64
	t.Cleanup(func() { t.Logf("vm: expect %d, ran %d", want.Load(), ran.Load()) })
	for _, path := range files {
		if !fixture.Shard(filepath.Base(path)) {
			continue
		}
		id := fixtureID(path)
		fx := manifest.ByID(id)
		if fx == nil {
			t.Fatalf("%s: no manifest entry (run UPDATE=1 go test -run TestManifestFresh ./internal/toolchain/driver/)", id)
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			if !countsOpenFds(t, path) {
				t.Parallel()
			}
			if !fx.Runs("vm") {
				t.Skipf("manifest: %s only runs native (%v)", id, fx.Skip)
			}
			want.Add(1)
			root, wantOut, wantErr, wantExit := loadRunFixture(t, path)
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"main/main.kg", "one", "two"}
			var out, errOut bytes.Buffer
			code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, args)
			if err != nil {
				t.Fatalf("manifest says vm runs %s, but it refused: %v", id, err)
			}
			ran.Add(1)
			if out.String() != wantOut {
				t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", out.String(), wantOut)
			}
			if err := fixture.CheckStderr(fx.Stderr, wantErr, errOut.String()); err != nil {
				t.Error(err)
			}
			if code != wantExit {
				t.Errorf("exit %d, want %d\nstderr:\n%s", code, wantExit, errOut.String())
			}
		})
	}
}

// countsOpenFds reports whether the fixture inspects /proc/self/fd, which in a VM run is the whole test process and so needs no sibling subtest running.
func countsOpenFds(t *testing.T, path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Contains(data, []byte("/proc/self/fd"))
}
