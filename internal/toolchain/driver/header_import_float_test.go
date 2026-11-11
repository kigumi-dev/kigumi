package driver_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A C struct with float members imports as a layout(C) record with f32 fields: four bytes each, aligned to four, laid out as C lays it out.
func TestAotHeaderImportFloatFields(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
	root, err := filepath.Abs("../../../testdata/native/header_import/float")
	if err != nil {
		t.Fatal(err)
	}
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KIGUMI_CFLAGS", filepath.Join(root, "helper.c"))
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Diagnostics(); d != "" {
		t.Fatalf("unexpected diagnostics:\n%s", d)
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	want := "made 3 0.1 0.5 0.2 0.3\n" +
		"scaled 4 0.3 1.5 0.6 0.90000004\n" +
		"sum 4.85 half 0.15\n" +
		"layout 24 24 8 8 4 16 20\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}
