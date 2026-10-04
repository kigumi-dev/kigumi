package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckReleasePolicy checks that a mod.local.kg-resolved dependency warns
// under development and fails under --release, with the same violation id in both.
func TestCheckReleasePolicy(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	dependency, _ := filepath.Abs("../../examples/dependency")

	code, _, errOut := execute(t, "--std", std, "check", dependency)
	if code != 0 {
		t.Fatalf("development check: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(errOut, "warning[release-local-replace]") {
		t.Fatalf("development check: want a release-local-replace warning, got %q", errOut)
	}

	code, errOut = executeReal(t, "--std", std, "check", "--release", dependency)
	if code != 1 {
		t.Fatalf("release check: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(errOut, "release-local-replace") {
		t.Fatalf("release check: want release-local-replace in the error, got %q", errOut)
	}
}

// `kigumi build --release` on a module with no release-policy violations
// accepts the flag and still emits IR.
func TestBuildReleaseEmitLLVM(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	hello, _ := filepath.Abs("../../examples/hello/main.kg")

	code, out, errOut := execute(t, "--std", std, "build", "--release", "--emit-llvm", hello)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "define") {
		t.Fatalf("expected LLVM IR on stdout, got %q", out)
	}
	if strings.Contains(errOut, "warning[") || strings.Contains(errOut, "error[") {
		t.Fatalf("a clean module should raise no release-policy violation, got %q", errOut)
	}
}

// --release on `kigumi build` rejects a dependency resolved through
// mod.local.kg before linking anything.
func TestBuildReleaseRejectsLocalReplace(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	dependency, _ := filepath.Abs("../../examples/dependency")

	code, errOut := executeReal(t, "--std", std, "build", "--release", "--emit-llvm", dependency)
	if code != 1 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(errOut, "release-local-replace") {
		t.Fatalf("want release-local-replace in the error, got %q", errOut)
	}
}
