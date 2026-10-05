package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestZeroBytesOverwritesTheSlice(t *testing.T) {
	b := []byte("super-secret-value")
	zeroBytes(b)
	for i, c := range b {
		if c != 0 {
			t.Fatalf("byte %d is %q, want 0", i, c)
		}
	}
}

// A handle opened before wipeAndRemove's unlink still reaches the same
// inode on POSIX, so it can see whether the overwrite landed before removal.
func TestWipeAndRemoveOverwritesBeforeUnlinking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kigumi-secret-probe")
	secret := []byte("super-secret-value")
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		t.Fatal(err)
	}
	held, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	wipeAndRemove(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("wipeAndRemove left %s behind: %v", path, err)
	}
	got := make([]byte, len(secret))
	if _, err := held.ReadAt(got, 0); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("secret")) {
		t.Fatalf("the unlinked inode still holds the secret: %q", got)
	}
	for i, c := range got {
		if c != 0 {
			t.Fatalf("byte %d of the unlinked inode is %q, want 0", i, c)
		}
	}
}
