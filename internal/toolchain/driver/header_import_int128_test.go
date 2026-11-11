package driver_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// A C struct with __int128 members imports as a layout(C) record with i128 / u128 fields; C and the shim agree on its size, alignment and offsets.
func TestAotHeaderImportInt128Fields(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
	if buildconfig.HostTarget().Arch != "amd64" && buildconfig.HostTarget().Arch != "arm64" {
		t.Skip("host C compiler may lack __int128")
	}
	root, err := filepath.Abs("../../../testdata/native/header_import/int128")
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
	want := "made 7 55340232221128654853 -55340232221128654853 11\n" +
		"bumped 8 55340232221128654854 -55340232221128654854 12\n" +
		"hi/lo 18446744073709551615 18446744073709551615\n" +
		"again 2 0 -6 -1\n" +
		"layout 64 64 16 16\n" +
		"scalar -6 340282366920938463444927863358058659840 -1 0\n" +
		"echo 0\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}
