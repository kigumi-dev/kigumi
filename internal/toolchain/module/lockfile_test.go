package module_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestLockRoundTrip(t *testing.T) {
	root := t.TempDir()
	lk := module.Lock{Entries: map[string]module.LockEntry{"b": {Version: "v2.0.0", Commit: "abc", Hash: "h1:1"}, "a": {Version: "v1.0.0", Hash: "h1:0"}}}
	if err := module.WriteLock(root, lk); err != nil {
		t.Fatal(err)
	}
	got, err := module.ReadLock(root)
	if err != nil || len(got.Entries) != 2 || got.Entries["b"] != lk.Entries["b"] || got.Entries["a"] != lk.Entries["a"] {
		t.Fatalf("read back %+v %v", got, err)
	}
	if err := module.WriteLock(root, module.Lock{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "mod.lock.kg")); !os.IsNotExist(err) {
		t.Fatal("empty lock should remove the file")
	}
	dir := filepath.Join(root, "tree")
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "x.kg"), []byte("a"), 0o644)
	h1, _ := module.TreeHash(dir)
	os.WriteFile(filepath.Join(dir, "sub", "x.kg"), []byte("b"), 0o644)
	h2, _ := module.TreeHash(dir)
	if h1 == h2 || !strings.HasPrefix(h1, "h1:") {
		t.Fatalf("tree hash should follow content: %s %s", h1, h2)
	}
}
