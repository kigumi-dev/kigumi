package session_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"kigumi/internal/cli/session"
)

func TestEmbeddedStdFallback(t *testing.T) {
	var s session.Session
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("KIGUMI_STD", "")
	session.SetEmbeddedStd(fstest.MapFS{
		"std/prelude/prelude.kg": {Data: []byte("pub type Ordering = Less | Equal | Greater\n")},
		"std/array/array.kg":     {Data: []byte("pub type Array[T]\n")},
	}, "std")
	defer session.SetEmbeddedStd(fstest.MapFS{}, ".")
	root := s.StdRoot()
	if root == "" {
		t.Fatal("no std root from the embedded copy")
	}
	if _, err := os.Stat(filepath.Join(root, "array", "array.kg")); err != nil {
		t.Fatalf("unpacked std incomplete: %v", err)
	}
	if again := s.StdRoot(); again != root {
		t.Fatalf("second lookup %q, want the cached %q", again, root)
	}
}

func TestEmbeddedStdRejectsPoisonedCache(t *testing.T) {
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

	os.RemoveAll(root)
	if err := os.MkdirAll(filepath.Join(root, "prelude"), 0o755); err != nil {
		t.Fatal(err)
	}
	poison := []byte("pub type Ordering = Poisoned\n")
	if err := os.WriteFile(filepath.Join(root, "prelude", "prelude.kg"), poison, 0o644); err != nil {
		t.Fatal(err)
	}

	again := s.StdRoot()
	if again != root {
		t.Fatalf("lookup after poisoning %q, want the same path %q", again, root)
	}
	got, err := os.ReadFile(filepath.Join(again, "prelude", "prelude.kg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "pub type Ordering = Less | Equal | Greater\n" {
		t.Fatalf("used a poisoned cache instead of re-extracting: %q", got)
	}
}
