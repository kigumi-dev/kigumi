package vcs

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestConcurrentClonesOfAnIncompleteDirNeverYieldAWrongTree(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	os.MkdirAll(repo, 0o755)
	gitOrSkip(t, repo, "init", "--quiet", "--initial-branch=main", ".")
	os.WriteFile(filepath.Join(repo, module.ManifestName), []byte("Module { name: \"example.com/dep\", kigumi: \"0.1\" }\n"), 0o644)
	for i := 0; i < 300; i++ {
		os.WriteFile(filepath.Join(repo, "f"+string(rune('a'+i%26))+string(rune('a'+i/26))+".kg"), []byte("pub fn f() -> Int {\n    1\n}\n"), 0o644)
	}
	gitOrSkip(t, repo, "add", ".")
	gitOrSkip(t, repo, "commit", "--quiet", "-m", "c")
	gitOrSkip(t, repo, "tag", "v1.0.0")
	r := module.Require{Name: "example.com/dep", Version: "v1.0.0", URL: "file://" + repo}

	ref := filepath.Join(t.TempDir(), "ref")
	if _, err := testGit(io.Discard).Clone(r, ref); err != nil {
		t.Fatal(err)
	}
	want, err := module.TreeHash(ref)
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 3; round++ {
		dir := filepath.Join(t.TempDir(), "mod", "dep@v1.0.0")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "junk"), []byte("x"), 0o644)
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := testGit(io.Discard).Clone(r, dir); err != nil {
					return
				}
				if got, err := module.TreeHash(dir); err == nil && got != want {
					t.Errorf("a successful clone hashed %s, want %s", got, want)
				}
			}()
		}
		wg.Wait()
		if got, err := module.TreeHash(dir); err != nil || got != want {
			t.Fatalf("round %d: the final entry hashes %s (%v), want %s", round, got, err, want)
		}
	}
}
