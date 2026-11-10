package build_test

import (
	devfixture "kigumi/internal/devkit/fixture"
	"os"
	"path/filepath"
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	"kigumi/internal/devkit/mirfixture"
	"kigumi/internal/devkit/semfixture"
)

// TestMirDump writes the MIR of every run fixture under MIRDUMP_DIR, so a
// change to the lowering can be compared byte for byte.
func TestMirDump(t *testing.T) {
	out := os.Getenv("MIRDUMP_DIR")
	if out == "" {
		t.Skip("set MIRDUMP_DIR to write one MIR dump per run fixture")
	}
	std := semfixture.LoadStd(t, "../../../../std")
	files, _ := filepath.Glob("../../../../testdata/run/*.txtar")
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	for _, path := range devfixture.Shards(files) {
		res, _, _ := mirfixture.CheckFixture(t, path, std)
		prog := mirbuild.Program(res)
		os.WriteFile(filepath.Join(out, filepath.Base(path)+".mir"), []byte(prog.Dump()+prog.Render()), 0o644)
	}
}
