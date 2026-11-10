package check_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// `kigumi check` reports build.kg's diagnostics next to the module's: a
// broken build program neither hides the module's errors nor is hidden by them.
func TestCheckReportsBuildFileDiagnostics(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mod.kg", "Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n")
	write("cmd/app/main.kg", "let n: String = 1\n")
	write("build.kg", "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let n: Int = \"x\"\n}\n")

	code, _, errOut := clitest.Execute(t, "--std", std, "check", root)
	if code != 1 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"cmd/app/main.kg", "build.kg"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("missing %q in:\n%s", want, errOut)
		}
	}
	if !strings.Contains(errOut, "expected `String`, found `Int`") || !strings.Contains(errOut, "expected `Int`, found `String`") {
		t.Errorf("both files' errors should be shown:\n%s", errOut)
	}
}

// A build.kg that does not parse is reported, and module checking still runs.
func TestCheckReportsBuildFileParseError(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let n: String = 1\n"), 0o644)
	os.WriteFile(filepath.Join(root, "build.kg"), []byte("import build from std/build\n\nfn configure(\n"), 0o644)

	code, _, errOut := clitest.Execute(t, "--std", std, "check", root)
	if code != 1 || !strings.Contains(errOut, "build.kg") || !strings.Contains(errOut, "main.kg") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

// A valid build.kg adds no output to a clean check.
func TestCheckAcceptsValidBuildFile(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let n = 1\nprint \"${n}\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "build.kg"), []byte("import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let app = b.executable(\"app\", \"\", b.request().target())?\n}\n"), 0o644)

	code, _, errOut := clitest.Execute(t, "--std", std, "check", root)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}
