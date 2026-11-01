package check_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestCheckReportsCapabilityNotProvided(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	file := filepath.Join(t.TempDir(), "main.kg")
	os.WriteFile(file, []byte("let args = host.args()\nprint \"${args.len()}\"\n"), 0o644)
	if code, _, errOut := clitest.Execute(t, "--std", std, "check", file); code != 0 {
		t.Fatalf("check on a hosted target: exit %d\n%s", code, errOut)
	}
	code, _, errOut := clitest.Execute(t, "--std", std, "--sys", "none", "check", file)
	if code != 1 || !strings.Contains(errOut, "E1370") || !strings.Contains(errOut, "host.args") {
		t.Fatalf("check with --sys none: exit %d\n%s", code, errOut)
	}
}
