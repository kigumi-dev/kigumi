package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/fixture"
	"kigumi/internal/devkit/mirfixture"
	"kigumi/internal/devkit/semfixture"
)

// Each testdata/mir fixture holds source sections and an `-- expect --`
// section with the IR dump followed by ownership diagnostics.
func TestMirCorpus(t *testing.T) {
	manifest, err := fixture.Load("../../../../testdata/manifest.json")
	if err != nil {
		t.Fatalf("load testdata/manifest.json: %v", err)
	}
	files, err := fixture.Corpus("../../../..", "mir")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no mir fixtures")
	}
	var ran atomic.Int64
	t.Cleanup(func() { t.Logf("mir: %d/%d fixtures passed", ran.Load(), len(files)) })
	for _, path := range fixture.ShardPaths(files) {
		id := fixture.IDFromPath(path)
		if manifest.ByID(id) == nil {
			t.Fatalf("%s: no manifest entry (run UPDATE=1 go test -run TestManifestFresh ./internal/toolchain/driver/)", id)
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			defer func() {
				if !t.Failed() {
					ran.Add(1)
				}
			}()
			runFixture(t, path, semfixture.LoadStd(t, "../../../../std"))
		})
	}
}

func runFixture(t *testing.T, path string, std []*sem.Package) {
	res, data, head := mirfixture.CheckFixture(t, path, std)
	prog := mirbuild.Program(res)
	// The contract holds for programs the builder accepted; a fixture that
	// expects ownership diagnostics is checked on its rendering alone.
	if !prog.HasErrors() {
		if err := mirverify.Program(prog); err != nil {
			t.Fatalf("verify: %v\n%s", err, prog.Dump())
		}
	}
	got := mirfixture.UserDump(prog) + prog.Render()
	want := ""
	if i := strings.Index(data, "-- expect --\n"); i >= 0 {
		want = data[i+len("-- expect --\n"):]
	}
	if os.Getenv("UPDATE") == "1" {
		os.WriteFile(path, []byte(head+"-- expect --\n"+got), 0o644)
		return
	}
	if got != want {
		t.Errorf("expect mismatch (UPDATE=1 to accept)\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
