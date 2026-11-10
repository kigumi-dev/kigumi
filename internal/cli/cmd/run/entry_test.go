package run_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// A module with no entry file is a target problem, not a VM refusal: `check` accepts the same module, so `run` must name
// the missing file instead of passing the interpreter's wording on.
func TestRunWithoutEntryFile(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"mod\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "only_test.kg"), []byte("import test from std/test\n\ntest \"t\" {\n    test.expectEq(1, 1)?\n}\n"), 0o644)

	code, _, errOut := clitest.Execute(t, "--std", std, "check", dir)
	if code != 0 {
		t.Fatalf("check of a module with no entry: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = clitest.Execute(t, "--std", std, "test", dir)
	if code != 0 {
		t.Fatalf("test of a module with no entry: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = clitest.Execute(t, "--std", std, "run", dir)
	if code != 1 {
		t.Fatalf("run without an entry: exit %d, stderr %q", code, errOut)
	}
	if strings.Contains(errOut, "vm: program has no entry") {
		t.Errorf("run leaked the interpreter's wording: %q", errOut)
	}
	if !strings.Contains(errOut, "no entry file") || !strings.Contains(errOut, "main.kg") {
		t.Errorf("run did not name the missing entry: %q", errOut)
	}
}

// Script mode (the shebang path `kigumi file.kg [args]`) forwards its arguments, but a literal "--" is the option
// terminator every entry mode consumes, so the script must not see it.
func TestScriptModeConsumesDashDash(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	file := filepath.Join(t.TempDir(), "s.kg")
	os.WriteFile(file, []byte(`let a = host.args()
for i in 1..a.len() {
    print "[${i}]=<${a.get(i) || "?"}>"
}
`), 0o644)

	code, out, errOut := clitest.Execute(t, "--std", std, file, "--", "a")
	if code != 0 {
		t.Fatalf("script mode: exit %d, stderr %q", code, errOut)
	}
	if want := "[1]=<a>\n"; out != want {
		t.Fatalf("script argv = %q, want %q", out, want)
	}
}
