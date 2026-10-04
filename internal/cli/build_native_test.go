package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/testtool"
)

// Split out from TestBuildRejectsBuildProgramFlagsWithoutOne so that one stays hermetic.
func TestBuildPlainBuildNative(t *testing.T) {
	name, version, ok := testtool.CCompiler()
	if !ok {
		t.Skip("no C toolchain (zig or clang) on PATH")
	}
	t.Logf("using toolchain: %s (%s)", name, version)

	std, _ := filepath.Abs("../../std")
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kg")
	os.WriteFile(file, []byte("fn main() -> Unit! {}\n"), 0o644)
	out := filepath.Join(dir, "a.out")

	if code, _, errOut := execute(t, "--std", std, "build", "-o", out, file); code != 0 {
		t.Fatalf("plain build without those flags should still work: exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("plain build did not produce %s: %v", out, err)
	}
}
