package mirfixture

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/devkit/fixture"
)

// SkipCompileError skips a run fixture whose manifest marks native as a
// compile error: MIR planning and verification only see programs that checked.
func SkipCompileError(t *testing.T, path string) {
	t.Helper()
	dir := "."
	for range 16 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		dir = filepath.Join(dir, "..")
	}
	m, err := fixture.Load(filepath.Join(dir, "testdata", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fx := m.ByID(fixture.IDFromPath(path)); fx != nil && fx.Skip["native"] == "compile-error" {
		t.Skipf("%s expects a compile error", fx.ID)
	}
}
