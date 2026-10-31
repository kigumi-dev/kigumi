package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/devkit/hosttool"
)

// Split out from TestBuildRejectsBuildProgramFlagsWithoutOne so that one stays hermetic.
func TestBuildPlainBuildNative(t *testing.T) {
	name, version, ok := hosttool.CCompiler(exec.LookPath)
	if !ok {
		t.Skip("no C toolchain (zig or clang) on PATH")
	}
	t.Logf("using toolchain: %s (%s)", name, version)

	std, _ := filepath.Abs("../../std")
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kg")
	os.WriteFile(file, []byte("fn main() -> Void! {}\n"), 0o644)
	out := filepath.Join(dir, "a.out")

	code, _, errOut := execute(t, "--std", std, "build", "-o", out, file)
	if code != 0 {
		t.Fatalf("plain build without those flags should still work: exit %d, stderr %q", code, errOut)
	}
	for _, line := range []string{"\n  linking: ", "\n  size: "} {
		if !strings.Contains(errOut, line) {
			t.Errorf("the build summary lacks %q:\n%s", strings.TrimSpace(line), errOut)
		}
	}
	if code, errOut := executeReal(t, "--std", std, "--target", "x86_64-linux-gnu", "build", "--link", "static", "-o", out, file); code == 0 || !strings.Contains(errOut, "--link static contradicts --target x86_64-linux-gnu") {
		t.Fatalf("--link static with --target: exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("plain build did not produce %s: %v", out, err)
	}
}
