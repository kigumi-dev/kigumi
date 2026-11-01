package session_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"kigumi/internal/cli/session"
)

// TestEmbeddedStdHashDistinguishesPathContentBoundary checks that two trees whose raw path||content bytes would collide still hash to different values.
func TestEmbeddedStdHashDistinguishesPathContentBoundary(t *testing.T) {
	var s session.Session

	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("KIGUMI_STD", "")
	session.SetEmbeddedStd(fstest.MapFS{
		"std/prelude/ab": {Data: []byte("c")},
	}, "std")
	rootA := s.StdRoot()
	if rootA == "" {
		t.Fatal("no std root for tree A")
	}

	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	session.SetEmbeddedStd(fstest.MapFS{
		"std/prelude/a": {Data: []byte("bc")},
	}, "std")
	defer session.SetEmbeddedStd(fstest.MapFS{}, ".")
	rootB := s.StdRoot()
	if rootB == "" {
		t.Fatal("no std root for tree B")
	}

	if filepath.Base(rootA) == filepath.Base(rootB) {
		t.Fatalf("different file trees must not share a cache directory name: %s vs %s", rootA, rootB)
	}
}

// TestEmbeddedStdNeverDeletesLiveDirectory checks that publishing over a stale cache entry never leaves the directory missing, even momentarily.
func TestEmbeddedStdNeverDeletesLiveDirectory(t *testing.T) {
	var s session.Session
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("KIGUMI_STD", "")
	session.SetEmbeddedStd(fstest.MapFS{
		"std/prelude/prelude.kg": {Data: []byte("pub type Ordering = Less | Equal | Greater\n")},
	}, "std")
	defer session.SetEmbeddedStd(fstest.MapFS{}, ".")
	root := s.StdRoot()
	if root == "" {
		t.Fatal("no std root from the embedded copy")
	}

	// Hold a handle into the about-to-be-poisoned tree, as a concurrent reader would; it must keep working after the directory entry changes.
	held, err := os.Open(filepath.Join(root, "prelude", "prelude.kg"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	poison := []byte("pub type Ordering = Poisoned\n")
	if err := os.WriteFile(filepath.Join(root, "prelude", "prelude.kg"), poison, 0o644); err != nil {
		t.Fatal(err)
	}

	again := s.StdRoot()
	if again != root {
		t.Fatalf("lookup after poisoning %q, want the same path %q", again, root)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("the directory must exist throughout republishing: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "prelude", "prelude.kg"))
	if err != nil || string(got) != "pub type Ordering = Less | Equal | Greater\n" {
		t.Fatalf("republished content wrong: %q, %v", got, err)
	}
	if _, err := held.Stat(); err != nil {
		t.Fatalf("a handle opened before republishing must stay valid: %v", err)
	}
}
