package test_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestTestCommandReportingFlags checks `kigumi test` wires --list and --filter through the CLI.
func TestTestCommandReportingFlags(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.kg"), []byte(`import test from std/test

test "alpha" {
    if 1 != 1 {
        fail test.failure("unreachable")
    }
}

test "beta" {
    fail test.failure("boom")
}
`), 0o644)

	code, out, errOut := clitest.Execute(t, "--std", std, "test", "--list", dir)
	if code != 0 {
		t.Fatalf("--list: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, `main::"alpha"`) || !strings.Contains(out, `main::"beta"`) {
		t.Fatalf("--list missing a stable id:\n%s", out)
	}

	code, out, errOut = clitest.Execute(t, "--std", std, "test", "--filter", "alpha", dir)
	if code != 0 || !strings.Contains(out, `ok   "alpha"`) || strings.Contains(out, "beta") {
		t.Fatalf("--filter alpha: exit %d\nstdout %q\nstderr %q", code, out, errOut)
	}

}

func TestTestListFilterMismatchFails(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.kg"), []byte("test \"alpha\" {\n}\n"), 0o644)
	code, _, errOut := clitest.Execute(t, "--std", std, "test", "--list", "--filter", "missing", dir)
	if code != 1 || !strings.Contains(errOut, "no test matches") {
		t.Fatalf("--list with a missing filter: exit %d stderr %q", code, errOut)
	}
}

func TestTestListJSON(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.kg"), []byte("test \"alpha\" {\n}\n"), 0o644)
	code, out, errOut := clitest.Execute(t, "--std", std, "test", "--list", "--json", dir)
	if code != 0 || !strings.Contains(out, `"type":"testList"`) || !strings.Contains(out, "alpha") || strings.Contains(out, "\t") {
		t.Fatalf("--list --json: exit %d stdout %q stderr %q", code, out, errOut)
	}
}
