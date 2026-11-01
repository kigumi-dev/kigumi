package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestPublishBuildOutputSucceeds covers the happy path: build's staged
// path receives the final content and out ends up holding it.
func TestPublishBuildOutputSucceeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "a.out")
	ok, err := Publish(out, func(staged string) error {
		return os.WriteFile(staged, []byte("new"), 0o755)
	})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "new" {
		t.Fatalf("out = %q, %v", got, err)
	}
}

// TestPublishBuildOutputKeepsLastGoodOnFailure checks that a failing build never destroys whatever good artifact already sat at out.
func TestPublishBuildOutputKeepsLastGoodOnFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "a.out")
	if err := os.WriteFile(out, []byte("GOOD"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err := Publish(out, func(staged string) error {
		os.WriteFile(staged, []byte("PARTIAL"), 0o644) // what a half-finished link might leave
		return errors.New("link failed")
	})
	if ok || err == nil {
		t.Fatalf("a failing build must report failure, got ok=%v err=%v", ok, err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "GOOD" {
		t.Fatalf("out must be untouched by the failed build, got %q, %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("the staging file must be cleaned up, got %v", entries)
	}
}

// TestPublishBuildOutputRejectsEmptyResult covers a build that reports
// success but produced nothing usable: out must still be left alone.
func TestPublishBuildOutputRejectsEmptyResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "a.out")
	if err := os.WriteFile(out, []byte("GOOD"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err := Publish(out, func(staged string) error {
		return nil // never wrote staged at all
	})
	if ok || err == nil {
		t.Fatalf("an empty/missing staged output must be reported as failure, got ok=%v err=%v", ok, err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "GOOD" {
		t.Fatalf("out must be untouched, got %q, %v", got, err)
	}
}
