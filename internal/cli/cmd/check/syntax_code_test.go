package check_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestSyntaxErrorsCarryTheCatalogCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.kg"), []byte("fn main( -> Void {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../../std")
	code, out, errOut := clitest.Execute(t, "--std", std, "check", dir)
	if code == 0 || !strings.Contains(out+errOut, "error[E2199]: expected identifier") {
		t.Fatalf("check must report a syntax error under E2199: exit %d\n%s%s", code, out, errOut)
	}
}
