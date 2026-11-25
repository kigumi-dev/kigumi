package module_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestLockViewRoundTrip(t *testing.T) {
	root := t.TempDir()
	state := sampleLockState(t, module.LockKindModule)
	if err := module.WriteLockState(root, state); err != nil {
		t.Fatal(err)
	}
	got, err := module.ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("read back %d entries, want 2: %+v", len(got.Entries), got.Entries)
	}
	named := got.Entries["example.com/lib/a"]
	if named.Name != "example.com/lib/a" || named.Alias != "" || named.Version != "v1.2.3" ||
		named.Commit != "sha1:"+strings.Repeat("a", 40) || named.Hash != lockDigest('3') {
		t.Fatalf("named entry = %+v", named)
	}
	aliased := got.Entries["libalias"]
	if aliased.Name != "example.com/lib/a" || aliased.Alias != "libalias" || aliased.Version != "v1.2.3" {
		t.Fatalf("aliased entry = %+v", aliased)
	}
	if r := aliased.Require(); r.Name != "example.com/lib/a" || r.Alias != "libalias" || r.Version != "v1.2.3" {
		t.Fatalf("Require() = %+v", r)
	}
}

func TestLockMissingIsEmptyView(t *testing.T) {
	root := t.TempDir()
	got, err := module.ReadLock(root)
	if err != nil {
		t.Fatalf("missing lock must read as an empty view: %v", err)
	}
	if len(got.Entries) != 0 {
		t.Fatalf("missing lock has %d entries", len(got.Entries))
	}
	if _, err := os.Stat(filepath.Join(root, module.LockName)); !os.IsNotExist(err) {
		t.Fatal("reading a missing lock must not create it")
	}
}

func TestLockZeroDependencyDocumentKept(t *testing.T) {
	root := t.TempDir()
	state := module.LockState{
		Kind:     module.LockKindModule,
		Format:   module.LockFormat1,
		Resolver: module.LockResolverOrderedV1,
		Inputs:   lockDigest('1'),
	}
	if err := module.WriteLockState(root, state); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(root, module.LockName))
	if err != nil {
		t.Fatalf("a zero-dependency lock must keep its file: %v", err)
	}
	if !strings.HasPrefix(string(src), `ModuleLock { format: "1"`) {
		t.Fatalf("lock does not start with the header:\n%s", src)
	}
	got, err := module.ReadLock(root)
	if err != nil || len(got.Entries) != 0 {
		t.Fatalf("read back %+v %v", got, err)
	}
}

func TestWriteLockStateRejectsIncompleteDocument(t *testing.T) {
	root := t.TempDir()
	err := module.WriteLockState(root, module.LockState{Kind: module.LockKindModule, Resolver: module.LockResolverOrderedV1})
	if err == nil {
		t.Fatal("wrote a document without a format or inputs")
	}
	if _, statErr := os.Stat(filepath.Join(root, module.LockName)); !os.IsNotExist(statErr) {
		t.Fatal("a rejected document must not be written")
	}
}

func TestReadLockStateMissingIsDistinct(t *testing.T) {
	root := t.TempDir()
	if _, err := module.ReadLockState(root, module.LockKindModule); !errors.Is(err, module.ErrLockMissing) {
		t.Fatalf("missing state error = %v, want ErrLockMissing", err)
	}
}

func TestTreeHashFollowsContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "x.kg"), []byte("a"), 0o644)
	h1, _ := module.TreeHash(dir)
	os.WriteFile(filepath.Join(dir, "sub", "x.kg"), []byte("b"), 0o644)
	h2, _ := module.TreeHash(dir)
	if h1 == h2 || !strings.HasPrefix(h1, "h1:") {
		t.Fatalf("tree hash should follow content: %s %s", h1, h2)
	}
}
