package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestBuildProgramCLI prints the plan of a build.kg-driven module and
// refuses the single-artifact flags with it.
func TestBuildProgramCLI(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	example, _ := filepath.Abs("../../../../examples/build_program")
	code, out, errOut := clitest.Execute(t, "--std", std, "build", "--plan", example, "--", "--big")
	if code != 0 || !strings.Contains(out, "\"big\"") || !strings.Contains(out, "\"order\"") {
		t.Fatalf("plan: exit %d\n%s%s", code, out, errOut)
	}
	if code, _, _ := clitest.Execute(t, "--std", std, "build", "-o", "x", example); code != 2 {
		t.Fatalf("-o with a build.kg should be a usage error, exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(example, "app")); err == nil {
		t.Fatal("--plan must not build anything")
	}
}

func TestBuildPlanExitsNonZeroForAnArtifactWithoutAnEntry(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	write := func(rel, body string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755)
		os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644)
	}
	write("mod.kg", "Module {\n    name: \"planentry\"\n    kigumi: \"0.1\"\n}\n")
	write("cmd/other/main.kg", "print \"x\"\n")
	write("build.kg", "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let a = b.executable(\"a\", \"cmd/nope\", b.request().target())?\n}\n")
	code, out, errOut := clitest.Execute(t, "--std", std, "build", dir, "--plan")
	if code == 0 || !strings.Contains(errOut, `no entry file under "cmd/nope"`) || strings.Contains(out, `"name": "a"`) {
		t.Fatalf("plan: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
