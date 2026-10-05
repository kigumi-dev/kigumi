package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// TestAotExportPanicAborts: a panic reaching an export(C) frame must abort
// before returning to the C caller; the C helper prints before and after, only "before" must reach stdout.
func TestAotExportPanicAborts(t *testing.T) {
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	src := `extern(C) {
    fn call_boundary(x: i32) -> i32
}

export(C) {
    pub fn kg_boundary_panic(x: i32) -> i32 {
        panic("boundary panic: ${x}")
    }
}

print "start"
let r = unsafe { call_boundary(5) }
print "unreachable ${r}"
`
	helper := filepath.Join(root, "helper.c")
	cHelper := "#include <stdio.h>\n" +
		"int kg_boundary_panic(int);\n" +
		"int call_boundary(int x) {\n" +
		"    printf(\"before\\n\");\n" +
		"    fflush(stdout);\n" +
		"    int r = kg_boundary_panic(x);\n" +
		"    printf(\"after\\n\");\n" +
		"    return r;\n" +
		"}\n"
	if err := os.WriteFile(helper, []byte(cHelper), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KIGUMI_CFLAGS", helper)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	ok, err := testBuild(m, exe, os.Stderr)
	if err != nil || !ok {
		t.Fatalf("build: %v", err)
	}
	cmd := exec.Command(exe)
	out, runErr := cmd.CombinedOutput()
	exitErr, isExit := runErr.(*exec.ExitError)
	if !isExit {
		t.Fatalf("want a non-zero exit, got %v\n%s", runErr, out)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2\n%s", exitErr.ExitCode(), out)
	}
	if !strings.Contains(string(out), "before\n") {
		t.Errorf("missing the C caller's pre-call output:\n%s", out)
	}
	if strings.Contains(string(out), "after") {
		t.Errorf("the C caller resumed after the panicking call; it must never regain control:\n%s", out)
	}
	if !strings.Contains(string(out), "panic: boundary panic: 5") {
		t.Errorf("missing the panic message:\n%s", out)
	}
}
