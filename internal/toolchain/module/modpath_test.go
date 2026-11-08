package module

import "testing"

func TestRepoOfURLWithPort(t *testing.T) {
	url, sub, ok := RepoOf(Require{Name: "git.example.com/team/repo/sub", URL: "git://git.example.com:2222/team/repo"})
	if !ok || url != "git://git.example.com:2222/team/repo" || sub != "sub" {
		t.Fatalf("RepoOf = %q, %q, %v; want the URL and subdir sub", url, sub, ok)
	}
}

func TestRepoOfScpURL(t *testing.T) {
	for _, url := range []string{"git@github.com:team/repo", "git@github.com:team/repo.git", "github.com:team/repo", "git@github.com:/team/repo.git", "git@github.com:/team/repo"} {
		_, sub, ok := RepoOf(Require{Name: "github.com/team/repo/sub", URL: url})
		if !ok || sub != "sub" {
			t.Errorf("RepoOf url %q: subdir %q, %v; want sub", url, sub, ok)
		}
	}
	_, sub, _ := RepoOf(Require{Name: "git.example.com/team/repo/sub", URL: "ssh://git@git.example.com:22/team/repo.git"})
	if sub != "sub" {
		t.Errorf("ssh url with port: subdir %q; want sub", sub)
	}
}
