package run_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/cli/exit"
)

// `run` documents [flags] before the entry file and must reject one placed after,
// which SetInterspersed(false) would otherwise forward to the script silently.
func TestRunFlagAfterEntryFile(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kg")
	os.WriteFile(file, []byte("print \"ok\"\n"), 0o644)

	code, _, errOut := clitest.Execute(t, "--std", std, "run", "--target", "bogus-triple-xyz", file)
	if code != exit.Value {
		t.Fatalf("--target before the entry file: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = clitest.Execute(t, "--std", std, "run", file, "--target", "bogus-triple-xyz")
	if code != exit.Usage || !strings.Contains(errOut, "--target") {
		t.Fatalf("--target after the entry file: exit %d, stderr %q", code, errOut)
	}
	code, out, _ := clitest.Execute(t, "--std", std, "run", file, "extra", "positional")
	if code != 0 || out != "ok\n" {
		t.Fatalf("plain args after the entry file: exit %d, stdout %q", code, out)
	}
}

// A literal "--" after the entry file ends the flag scan, so a script argument
// shaped like a kigumi flag (e.g. "--target") reaches the script.
func TestRunDashDashForwardsScriptArgv(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	file := filepath.Join(dir, "main.kg")
	os.WriteFile(file, []byte(`let args = host.args()
for i in 1..args.len() {
    print "${args.get(i) || "?"}"
}
`), 0o644)

	code, out, errOut := clitest.Execute(t, "--std", std, "run", file, "--", "--target", "literal")
	if code != 0 {
		t.Fatalf("run with -- before flag-shaped argv: exit %d, stderr %q", code, errOut)
	}
	if want := "--target\nliteral\n"; out != want {
		t.Fatalf("script argv after --: got %q, want %q", out, want)
	}
}
