package doc_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"kigumi/internal/tooling/doc"
)

func noValidate(string) error { return nil }

func TestPublishDedicated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "site")
	files := map[string][]byte{"index.html": []byte("one")}
	if err := doc.Publish(dir, files, noValidate); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "index.html")); err != nil || string(b) != "one" {
		t.Errorf("index.html: %q, %v", b, err)
	}
}

func TestPublishDedicatedDropsStalePage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "site")
	if err := doc.Publish(dir, map[string][]byte{"old.html": []byte("old"), "index.html": []byte("i")}, noValidate); err != nil {
		t.Fatal(err)
	}
	if err := doc.Publish(dir, map[string][]byte{"index.html": []byte("i2")}, noValidate); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old.html")); !os.IsNotExist(err) {
		t.Errorf("old.html should be gone, stat err = %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "index.html")); string(b) != "i2" {
		t.Errorf("index.html = %q", b)
	}
}

func TestPublishMixedRemovesEmptyPackageDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("user content"), 0o644)
	if err := doc.Publish(dir, map[string][]byte{"std/crypto/subtle.html": []byte("x"), "index.html": []byte("i")}, noValidate); err != nil {
		t.Fatal(err)
	}
	if err := doc.Publish(dir, map[string][]byte{"index.html": []byte("i2")}, noValidate); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "std", "crypto")); !os.IsNotExist(err) {
		t.Errorf("std/crypto should be gone, stat err = %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "README.md")); err != nil || string(b) != "user content" {
		t.Errorf("README.md was touched: %q, %v", b, err)
	}
}

// isDedicated must resolve the root symlink before walking it, or Lstat on
// the symlink itself stops the walk before it reaches the real contents.
func TestPublishDedicatedThroughSymlink(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "real_target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	site := filepath.Join(base, "site")
	if err := os.Symlink(target, site); err != nil {
		t.Fatal(err)
	}
	if err := doc.Publish(site, map[string][]byte{"index.html": []byte("one")}, noValidate); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(site)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("site is still a symlink: isDedicated should have made this a swapIn, replacing the top-level entry")
	}
	if b, err := os.ReadFile(filepath.Join(site, "index.html")); err != nil || string(b) != "one" {
		t.Errorf("index.html: %q, %v", b, err)
	}
}

func TestPublishMixedKeepsUserFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("user content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := doc.Publish(dir, map[string][]byte{"api.html": []byte("v1")}, noValidate); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "README.md")); err != nil || string(b) != "user content" {
		t.Errorf("README.md was touched: %q, %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "api.html")); err != nil || string(b) != "v1" {
		t.Errorf("api.html: %q, %v", b, err)
	}

	if err := doc.Publish(dir, map[string][]byte{"api2.html": []byte("v2")}, noValidate); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "api.html")); !os.IsNotExist(err) {
		t.Errorf("a page the new run no longer emits should be removed, stat err = %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "api2.html")); err != nil || string(b) != "v2" {
		t.Errorf("api2.html: %q, %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "README.md")); err != nil || string(b) != "user content" {
		t.Errorf("README.md was touched on the second publish: %q, %v", b, err)
	}
}

func TestPublishConcurrentStagingDoesNotCollide(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "site")
	var mu sync.Mutex
	var stagingDirs []string
	var staged sync.WaitGroup
	staged.Add(2)
	start := make(chan struct{})
	record := func(staging string) error {
		mu.Lock()
		stagingDirs = append(stagingDirs, staging)
		mu.Unlock()
		staged.Done()
		<-start
		return nil
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = doc.Publish(dir, map[string][]byte{"index.html": []byte("x")}, record)
		}(i)
	}
	staged.Wait()
	close(start)
	wg.Wait()
	if stagingDirs[0] == stagingDirs[1] {
		t.Fatalf("both publishes used the same staging directory: %q", stagingDirs[0])
	}
}

func TestPublishValidationFailureLeavesDirUntouched(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "site")
	failing := func(string) error { return os.ErrInvalid }
	if err := doc.Publish(dir, map[string][]byte{"index.html": []byte("x")}, failing); err == nil {
		t.Fatal("expected the failing validator to stop the publish")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dir should not have been created, stat err = %v", err)
	}
}

func TestPublishLeavesSiblingOldDirectory(t *testing.T) {
	parent := t.TempDir()
	keep := filepath.Join(parent, "site.old", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	site := filepath.Join(parent, "site")
	for range 2 {
		if err := doc.Publish(site, map[string][]byte{"index.html": []byte("one")}, noValidate); err != nil {
			t.Fatal(err)
		}
	}
	if b, err := os.ReadFile(keep); err != nil || string(b) != "x" {
		t.Errorf("an unrelated site.old directory was disturbed: %q, %v", b, err)
	}
	left, _ := filepath.Glob(filepath.Join(parent, "site.*"))
	if len(left) != 1 {
		t.Errorf("publish left stray directories behind: %v", left)
	}
}

func TestPublishCreatesParentsWithPublicMode(t *testing.T) {
	site := filepath.Join(t.TempDir(), "deep", "er", "site")
	if err := doc.Publish(site, map[string][]byte{"index.html": []byte("one")}, noValidate); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(site)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("fresh output directory mode = %o, want 755", got)
	}
}
