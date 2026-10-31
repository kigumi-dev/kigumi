package driver_test

import (
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func TestLockRoundTripsUrl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mod.lock.kg")
	want := driver.Lock{Entries: map[string]driver.LockEntry{
		"a": {Version: "v1.0.0", Commit: "abc", Hash: "h1:x", URL: "https://example.com/a"},
		"b": {Version: "v1.0.0", Commit: "def", Hash: "h1:y"},
	}}
	if err := driver.WriteLockFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := driver.ReadLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Entries["a"].URL != "https://example.com/a" || got.Entries["b"].URL != "" {
		t.Fatalf("got %+v", got.Entries)
	}
}
