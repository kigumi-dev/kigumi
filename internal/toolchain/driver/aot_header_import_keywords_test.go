package driver_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func TestAotHeaderImportKeywordNames(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
	root, err := filepath.Abs("../../../testdata/cheader_keywords")
	if err != nil {
		t.Fatal(err)
	}
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KIGUMI_CFLAGS", filepath.Join(root, "kw.c"))
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
	if want := "36 105 42 3 4\n"; string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}
