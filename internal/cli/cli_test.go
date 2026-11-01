package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestCommands(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	code, _, _ := clitest.Execute(t, "--std", std, "check", "../../examples/repostat")
	if code != 0 {
		t.Fatalf("check repostat: exit %d", code)
	}
	if code, out, _ := clitest.Execute(t, "--std", std, "../../examples/hello/main.kg"); code != 0 || out != "Hello, Kigumi!\n" {
		t.Fatalf("run root-level entry: exit %d, stdout %q", code, out)
	}
	if code, _, errOut := clitest.Execute(t, "--std", std, "check", "../../examples/repostat/cmd/repostat/main.kg"); code != 0 {
		t.Fatalf("check repostat entry file: exit %d, stderr %q", code, errOut)
	}
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "main"), 0o755)
	file := filepath.Join(dir, "main", "main.kg")
	os.WriteFile(file, []byte("let x: String = 1\nprint x\n"), 0o644)
	code, _, errOut := clitest.Execute(t, "--std", std, "check", file)
	if code != 1 || !strings.Contains(errOut, "expected `String`, found `Int`") {
		t.Fatalf("check with type error: exit %d, stderr %q", code, errOut)
	}
	os.WriteFile(file, []byte("let  x = 1\nprint \"v=${x}\"\n"), 0o644)
	code, _, errOut = clitest.Execute(t, "fmt", "--check", file)
	if code != 1 || !strings.Contains(errOut, "not formatted") {
		t.Fatalf("fmt --check: exit %d, stderr %q", code, errOut)
	}
	code, out, _ := clitest.Execute(t, "--std", std, "run", file, "extra")
	if code != 0 || out != "v=1\n" {
		t.Fatalf("run: exit %d, stdout %q", code, out)
	}
	code, out, _ = clitest.Execute(t, "--std", std, file, "--not-a-flag")
	if code != 0 || out != "v=1\n" {
		t.Fatalf("run via bare file: exit %d, stdout %q", code, out)
	}
	if code, out, _ = clitest.Execute(t); code != 0 || !strings.Contains(out, "Usage:") {
		t.Fatalf("no arguments: exit %d, stdout %q", code, out)
	}
	if code, _, _ = clitest.Execute(t, "bogus"); code != 2 {
		t.Fatalf("unknown command: exit %d", code)
	}
	if code, _, _ = clitest.Execute(t, "check"); code != 2 {
		t.Fatalf("missing argument: exit %d", code)
	}
	if code, _, _ = clitest.Execute(t, "--bogus"); code != 2 {
		t.Fatalf("unknown flag: exit %d", code)
	}
	if code, _, _ = clitest.Execute(t, "check", filepath.Join(dir, "missing")); code != 1 {
		t.Fatalf("missing module: exit %d", code)
	}
}

// A nonexistent .kg entry file must fail like a nonexistent module root does,
// not load an empty or unrelated module and report success.
func TestMissingEntryFile(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "other.kg"), []byte("fn main() -> Void! {}\n"), 0o644)
	missing := filepath.Join(dir, "does_not_exist.kg")

	for _, sub := range []string{"check", "test", "doc"} {
		code, _, errOut := clitest.Execute(t, "--std", std, sub, missing)
		if code != 1 || !strings.Contains(errOut, "no such file") {
			t.Errorf("%s on missing entry file: exit %d, stderr %q", sub, code, errOut)
		}
	}

	out := filepath.Join(dir, "a.out")
	code, _, errOut := clitest.Execute(t, "--std", std, "build", "-o", out, missing)
	if code != 1 || !strings.Contains(errOut, "no such file") {
		t.Errorf("build on missing entry file: exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("build on missing entry file wrote an output binary")
	}

	code, _, errOut = clitest.Execute(t, "--std", std, "run", missing)
	if code != 1 || !strings.Contains(errOut, "no such file") {
		t.Errorf("run on missing entry file: exit %d, stderr %q", code, errOut)
	}
	if strings.Contains(errOut, "top-level statements") || strings.Contains(errOut, "no entry") {
		t.Errorf("run on missing entry file leaked an unrelated diagnostic: %q", errOut)
	}
}
