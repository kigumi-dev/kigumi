package module

import "testing"

func TestRepoOfURLWithPort(t *testing.T) {
	url, sub, ok := RepoOf(Require{Name: "git.example.com/team/repo/sub", URL: "git://git.example.com:2222/team/repo"})
	if !ok || url != "git://git.example.com:2222/team/repo" || sub != "sub" {
		t.Fatalf("RepoOf = %q, %q, %v; want the URL and subdir sub", url, sub, ok)
	}
}
