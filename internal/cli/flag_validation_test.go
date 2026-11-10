package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/cli/exit"
)

// An unrecognized --color value must be a usage error rather than
// silently falling back to auto-detection.
func TestColorFlagValidation(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	hello, _ := filepath.Abs("../../examples/hello/main.kg")

	if code, _, errOut := clitest.Execute(t, "--color", "banana", "--std", std, "run", hello); code != 2 || !strings.Contains(errOut, "--color") {
		t.Fatalf("--color banana: exit %d, stderr %q", code, errOut)
	}
	if code, out, errOut := clitest.Execute(t, "--color", "never", "--std", std, "run", hello); code != 0 || out != "Hello, Kigumi!\n" {
		t.Fatalf("--color never: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestBuildLinkFlagValidation(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	hello, _ := filepath.Abs("../../examples/hello/main.kg")
	out := filepath.Join(t.TempDir(), "a.out")
	if code, _, errOut := clitest.Execute(t, "--std", std, "build", "--link", "banana", "-o", out, hello); code != 2 || !strings.Contains(errOut, "link") {
		t.Fatalf("--link banana: exit %d, stderr %q", code, errOut)
	}
}

// `build --plan/--out-dir/--secret` only mean anything for a module whose
// build.kg has a Program.
func TestBuildRejectsBuildProgramFlagsWithoutOne(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kg")
	os.WriteFile(file, []byte("fn main() -> Void! {}\n"), 0o644)
	out := filepath.Join(dir, "a.out")

	if code, _, errOut := clitest.Execute(t, "--std", std, "build", "--plan", "-o", out, file); code != exit.Usage || !strings.Contains(errOut, "--plan") {
		t.Fatalf("build --plan without build.kg: exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("build --plan without build.kg must not build anything")
	}
	if code, _, errOut := clitest.Execute(t, "--std", std, "build", "--out-dir", dir, "-o", out, file); code != exit.Usage || !strings.Contains(errOut, "--out-dir") {
		t.Fatalf("build --out-dir without build.kg: exit %d, stderr %q", code, errOut)
	}
	if code, _, errOut := clitest.Execute(t, "--std", std, "build", "--secret", "x=/nonexistent", "-o", out, file); code != exit.Usage || !strings.Contains(errOut, "--secret") {
		t.Fatalf("build --secret without build.kg: exit %d, stderr %q", code, errOut)
	}
}

// `fmt --check -w` must be a usage error rather than silently picking
// --check.
func TestFmtCheckWriteMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kg")
	src := "let   x = 1\nprint x\n"
	os.WriteFile(file, []byte(src), 0o644)

	code, _, errOut := clitest.Execute(t, "fmt", "--check", "-w", file)
	if code != exit.Usage || !strings.Contains(errOut, "--check") || !strings.Contains(errOut, "--write") {
		t.Fatalf("fmt --check -w: exit %d, stderr %q", code, errOut)
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != src {
		t.Fatalf("fmt --check -w must not touch the file: %q, %v", got, err)
	}
}
