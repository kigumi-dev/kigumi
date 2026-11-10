package fmt_test

import (
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// fmt reports each failing file itself and then exits, so it must prefix its
// errors the way exit.Execute renders the ones it returns.
func TestFmtErrorsCarryTheKigumiPrefix(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.kg")
	code, _, errOut := clitest.Execute(t, "fmt", missing)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, errOut)
	}
	if !strings.HasPrefix(errOut, "kigumi: ") {
		t.Errorf("stderr should start with the kigumi: prefix, got %q", errOut)
	}
}
