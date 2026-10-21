package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainFindsSyntaxAndOwnershipCodes(t *testing.T) {
	for _, tc := range []struct{ arg, code, name string }{
		{"E2199", "E2199", "syntax-error"},
		{"syntax-error", "E2199", "syntax-error"},
		{"E2197", "E2197", "use-after-move"},
		{"use-after-move", "E2197", "use-after-move"},
		{"E2198", "E2198", "borrow-conflict"},
		{"E716", "E716", "defer-fail-outside-result"},
	} {
		code, out, errOut := execute(t, "explain", tc.arg)
		if code != 0 || !strings.HasPrefix(out, tc.code+" ("+tc.name+")") {
			t.Errorf("explain %s: exit %d\n%s%s", tc.arg, code, out, errOut)
		}
	}
}

func TestSyntaxErrorsCarryTheCatalogCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.kg"), []byte("fn main( -> Void {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	code, out, errOut := execute(t, "--std", std, "check", dir)
	if code == 0 || !strings.Contains(out+errOut, "error[E2199]: expected identifier") {
		t.Fatalf("check must report a syntax error under E2199: exit %d\n%s%s", code, out, errOut)
	}
}
