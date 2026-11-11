package driver_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// <stdlib.h>'s exit/abort/_Exit/quick_exit are `noreturn`. zig translate-c
// spells that result `noreturn`; the importer maps it to Never, so the
// declarations stay visible and a Never tail can stand in for the caller's
// result type. A skipped declaration only surfaces as the E994 warning that
// names it, which is what this fixture would regress into.
func TestHeaderImportNoreturnResults(t *testing.T) {
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
	root, err := filepath.Abs("../../../testdata/native/header_import/noreturn")
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
	d := m.Diagnostics()
	for _, bad := range []string{"unsupported return type in `abort`", "unsupported return type in `exit`", "no visible `abort`", "no visible `exit`"} {
		if strings.Contains(d, bad) {
			t.Errorf("diagnostics mention %q:\n%s", bad, d)
		}
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("a noreturn result must coerce like Never:\n%s", a.Res.Render())
	}
}
