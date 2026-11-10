package driver_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

// testdata/cheader_abi_shapes/h.h holds the header shapes that used to be
// dropped whole: a self-referential struct, a mutually recursive pair, a
// forward typedef, `#pragma pack`/packed/aligned records (with the field
// align(N) annotations zig translate-c emits for them) and named
// function-pointer parameters. The imported layouts match C's sizeof/alignof.
func TestHeaderImportABIShapes(t *testing.T) {
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
	root, err := filepath.Abs("../../../testdata/cheader_abi_shapes")
	if err != nil {
		t.Fatal(err)
	}
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	if m.HasParseErrors() {
		t.Fatalf("generated header package does not parse:\n%s", m.Diagnostics())
	}
	if d := m.Diagnostics(); strings.Contains(d, "E994") {
		t.Fatalf("a header shape was skipped:\n%s", d)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	var out, errOut bytes.Buffer
	if code, err := vm.Run(a.Prog, vm.Options{}, &out, &errOut); err != nil || code != 0 {
		t.Fatalf("run: %v code=%d %s", err, code, errOut.String())
	}
	// P1 (pack(1)) and P2 (packed) are 5 bytes aligned 1; A16 is 16/16.
	if want := "5/1 5/1 16/16\n"; out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}
