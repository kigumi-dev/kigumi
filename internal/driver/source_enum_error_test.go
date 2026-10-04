package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// TestSourceDirectoryPermissionErrorFailsLoad checks that a Source directory ReadDir can't list fails the module load with the module and path named.
func TestSourceDirectoryPermissionErrorFailsLoad(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"native\"\n    kigumi: \"0.1\"\n}\nSource { c: \"native\" }\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"x\"\n"), 0o644)
	nativeDir := filepath.Join(root, "native")
	os.MkdirAll(nativeDir, 0o755)
	os.WriteFile(filepath.Join(nativeDir, "x.c"), nil, 0o644)
	if err := os.Chmod(nativeDir, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(nativeDir, 0o755)

	std, _ := filepath.Abs("../../std")
	_, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err == nil {
		t.Fatal("an unreadable Source directory must fail the load, not silently contribute zero sources")
	}
	if !strings.Contains(err.Error(), "native") {
		t.Fatalf("error must name the module/Source path, got: %v", err)
	}
}

// TestSourceEmptyDirectoryIsZeroSources checks that a genuinely empty directory is a normal empty result, not an error.
func TestSourceEmptyDirectoryIsZeroSources(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"native\"\n    kigumi: \"0.1\"\n}\nSource { c: \"empty\" }\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"x\"\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "empty"), 0o755)

	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sources) != 0 {
		t.Fatalf("an empty Source directory must contribute zero sources, got %v", m.Sources)
	}
}
