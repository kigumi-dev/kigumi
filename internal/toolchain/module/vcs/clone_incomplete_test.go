package vcs

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestCloneReplacesAnIncompleteCacheDirectory(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	os.MkdirAll(repo, 0o755)
	gitOrSkip(t, repo, "init", "--quiet", "--initial-branch=main", ".")
	os.WriteFile(filepath.Join(repo, module.ManifestName), []byte("Module { name: \"example.com/dep\", kigumi: \"0.1\" }\n"), 0o644)
	gitOrSkip(t, repo, "add", ".")
	gitOrSkip(t, repo, "commit", "--quiet", "-m", "c")
	gitOrSkip(t, repo, "tag", "v1.0.0")

	dir := filepath.Join(t.TempDir(), "mod", "dep@v1.0.0")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "junk"), []byte("x"), 0o644)
	r := module.Require{Name: "example.com/dep", Version: "v1.0.0", URL: "file://" + repo}
	if _, err := testGit(io.Discard).Clone(r, dir); err != nil {
		t.Fatal(err)
	}
	if !module.CacheReady(dir) {
		t.Fatal("the entry must be complete after the clone")
	}
	if _, err := os.Stat(filepath.Join(dir, "junk")); err == nil {
		t.Fatal("the leftover file must be gone")
	}
}
