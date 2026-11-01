package mirfixture

import (
	"os"
	"path/filepath"
	"testing"
)

// stdDir is the std tree beside the go.mod nearest above the package directory go test runs in.
func stdDir(t testing.TB) string {
	t.Helper()
	dir := "."
	for range 16 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "std")
		}
		dir = filepath.Join(dir, "..")
	}
	t.Fatal("no go.mod above the package directory")
	return ""
}
