package explain_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/devkit/hosttool"
)

const capabilitySrc = "let args = host.args()\nprint \"${args.len()}\"\n"

func capabilityModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kg")
	os.WriteFile(file, []byte(capabilitySrc), 0o644)
	return file
}

func TestExplainCapabilitiesResolvesAgainstTarget(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	file := capabilityModule(t)

	code, out, errOut := clitest.Execute(t, "--std", std, "explain", "--capabilities", file)
	if code != 0 || !strings.Contains(out, "host.args: provided") || !strings.Contains(out, "HOST_ARGS (entry)") {
		t.Fatalf("explain --capabilities on a hosted target: exit %d\n%s%s", code, out, errOut)
	}

	code, out, errOut = clitest.Execute(t, "--std", std, "--sys", "none", "explain", "--capabilities", file)
	if code != 0 || !strings.Contains(out, "host.args: withheld, ") || !strings.Contains(out, "--sys none") {
		t.Fatalf("explain --capabilities with --sys none: exit %d\n%s%s", code, out, errOut)
	}
}

func TestExplainCapabilitiesReadsBuiltArtifact(t *testing.T) {
	if _, _, ok := hosttool.CCompiler(exec.LookPath); !ok {
		t.Skip("no C toolchain (zig or clang) on PATH")
	}
	std, _ := filepath.Abs("../../../../std")
	file := capabilityModule(t)
	out := filepath.Join(filepath.Dir(file), "a.out")
	if code, _, errOut := clitest.Execute(t, "--std", std, "build", "-o", out, file); code != 0 {
		t.Fatalf("build: exit %d, stderr %q", code, errOut)
	}
	os.Remove(file)

	code, got, errOut := clitest.Execute(t, "explain", "--capabilities", out)
	if code != 0 || !strings.HasPrefix(got, "artifact "+out) || !strings.Contains(got, "host.args: provided") || !strings.Contains(got, "HOST_ARGS (entry)") {
		t.Fatalf("explain --capabilities on a built artifact: exit %d\n%s%s", code, got, errOut)
	}
}
