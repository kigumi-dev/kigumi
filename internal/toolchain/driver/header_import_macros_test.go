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

// INT16_MIN is -32768, not -32767; macros that cannot be computed are skipped under E994.
func TestHeaderImportMacroValues(t *testing.T) {
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
	root, err := filepath.Abs("../../../testdata/native/header_import/macros")
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
	d := m.Diagnostics()
	for _, name := range []string{"E994", "OVERFLOWS", "DIV_ZERO", "BAD_BYTE", "WIDE_L", "WIDE_U16", "WIDE_U32", "WIDE_MID", "WIDE_COMMENTED"} {
		if !strings.Contains(d, name) {
			t.Errorf("diagnostic does not mention %q:\n%s", name, d)
		}
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
	want := "-128 -32768 -2147483648 -9223372036854775808 -9223372036854775808 -2147483648\n" +
		"-6 2147483648 -2147483648 -32768 100000000000 65535 14\n" +
		"hello, world 4294967295\n" +
		"0 0 0 2147483648 0 true true cost: ${x}\n" +
		"9223372036854775807 -9223372036854775808 18446744073709551615 9223372036854775807 -9223372036854775808 18446744073709551615\n" +
		"9223372036854775807 18446744073709551615 5 -5\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}
