package fsx_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/base/fsx"
)

// WriteFileAtomic (used by `kigumi fmt -w`) preserves an existing target's
// mode, falling back to the given perm only when the file does not exist yet.
func TestWriteFileAtomicPreservesExistingMode(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := fsx.WriteFileAtomic(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fsx.WriteFileAtomic(path, []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("existing mode not preserved: got %v", fi.Mode().Perm())
	}
}
