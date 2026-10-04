package get

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"kigumi/internal/errors"
)

func gitOrSkip(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.email=t@example.com", "-c", "user.name=t"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// repoWithHistory builds a repo with n commits on main and returns its path
// and the HEAD commit's full hash.
func repoWithHistory(t *testing.T, n int) (string, string) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	os.MkdirAll(repo, 0o755)
	gitOrSkip(t, repo, "init", "--quiet", "--initial-branch=main", ".")
	for i := 0; i < n; i++ {
		os.WriteFile(filepath.Join(repo, "a.txt"), []byte(strings.Repeat("x", i+1)), 0o644)
		gitOrSkip(t, repo, "add", "a.txt")
		gitOrSkip(t, repo, "commit", "--quiet", "-m", "c")
	}
	head := gitOrSkip(t, repo, "rev-parse", "HEAD")
	return repo, head
}

// Unlike cloneCommitFull, shallowFetchCommit must never pull the rest of the repository's history.
func TestShallowFetchCommitIsBounded(t *testing.T) {
	repo, head := repoWithHistory(t, 5)
	tmp := t.TempDir()
	cmd := &cobra.Command{}
	full, err := shallowFetchCommit(cmd, repo, head, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if full != head {
		t.Fatalf("resolved %s, want %s", full, head)
	}
	count := gitOrSkip(t, tmp, "rev-list", "--count", "HEAD")
	if count != "1" {
		t.Errorf("shallowFetchCommit pulled %s commits, want 1", count)
	}
}

// cloneCommit must still resolve the commit, visibly, when the bounded fetch fails.
func TestCloneCommitFallsBackWhenShallowFetchFails(t *testing.T) {
	repo, head := repoWithHistory(t, 3)
	tmp := t.TempDir()
	prev := shallowFetch
	shallowFetch = func(cmd *cobra.Command, url, hash, tmp string) (string, error) {
		return "", errors.NewErrf("server does not support fetching an arbitrary commit")
	}
	defer func() { shallowFetch = prev }()
	var out strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	full, err := cloneCommit(cmd, repo, head, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if full != head {
		t.Fatalf("resolved %s, want %s", full, head)
	}
	if !strings.Contains(out.String(), "cloning the full history instead") {
		t.Errorf("the fallback to a full clone must be visible:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "a.txt")); err != nil {
		t.Fatalf("checkout missing after the fallback: %v", err)
	}
}

// Exercises cloneCommit's whole path (shallow attempt, then checkout) where the shallow attempt is expected to succeed.
func TestCloneCommitSucceedsOnPlainHistory(t *testing.T) {
	repo, head := repoWithHistory(t, 4)
	tmp := t.TempDir()
	os.RemoveAll(tmp)
	os.MkdirAll(tmp, 0o755)
	var out strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	full, err := cloneCommit(cmd, repo, head, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if full != head {
		t.Fatalf("resolved %s, want %s", full, head)
	}
	if strings.Contains(out.String(), "cloning the full history instead") {
		t.Errorf("a server that supports a direct commit fetch should not fall back:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "a.txt")); err != nil {
		t.Fatalf("checkout missing: %v", err)
	}
}
