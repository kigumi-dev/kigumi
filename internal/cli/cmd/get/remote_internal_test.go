package get

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func TestCheckRemote(t *testing.T) {
	good := []string{"https://github.com/a/b.git", "ssh://git@host/a/b", "git://host/a/b", "file:///tmp/x", "git@github.com:a/b.git"}
	bad := []string{"", "--upload-pack=touch X", "-oProxyCommand=x", "ext::sh -c touch% X", "/tmp/x", "http://host/a", "ssh://-oProxyCommand=x/a", "ssh://-oProxyCommand=true@example.com/p", "https:///a", "ftp://host/a", "host:path", "https://h/a\nb"}
	for _, u := range good {
		if err := checkRemote(u); err != nil {
			t.Errorf("%q rejected: %v", u, err)
		}
	}
	for _, u := range bad {
		err := checkRemote(u)
		if err == nil {
			t.Errorf("%q accepted", u)
		} else if !strings.Contains(err.Error(), "refusing repository url") {
			t.Errorf("%q: error does not name the url: %v", u, err)
		}
	}
}

func TestCheckRequireRejectsDashVersion(t *testing.T) {
	r := driver.Require{Name: "example.com/x", Version: "-v1"}
	if err := checkRequire(r, "https://example.com/x.git"); err == nil {
		t.Fatal("a version starting with `-` must be refused")
	}
}

// Even when a url reaches a git call unchecked, `--` keeps it a repository argument.
func TestGitCallsTreatURLAsArgument(t *testing.T) {
	root := t.TempDir()
	mark := filepath.Join(root, "MARK")
	evil := "--upload-pack=touch " + mark
	quiet := session.IO{Err: &strings.Builder{}}
	cloneTag(quiet, evil, "v1.0.0", filepath.Join(root, "a"))
	os.MkdirAll(filepath.Join(root, "b"), 0o755)
	shallowFetchCommit(quiet, evil, strings.Repeat("a", 40), filepath.Join(root, "b"))
	os.MkdirAll(filepath.Join(root, "c"), 0o755)
	cloneCommitFull(quiet, evil, strings.Repeat("a", 40), filepath.Join(root, "c"))
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("a url that looks like an option ran a command")
	}
}

func TestConfinedSubdirStaysInsideCheckout(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "a", "b"), 0o755)
	r := driver.Require{Name: "example.com/m", Version: "v1.0.0"}
	if got, err := confinedSubdir(tmp, "a/b", r); err != nil || !strings.HasSuffix(got, filepath.Join("a", "b")) {
		t.Fatalf("a plain subdirectory: %q %v", got, err)
	}
	for _, sub := range []string{"../..", "a/../../.."} {
		if _, err := confinedSubdir(tmp, sub, r); err == nil || !strings.Contains(err.Error(), "outside") {
			t.Errorf("%q must be refused, got %v", sub, err)
		}
	}
}

func TestGitProtocolsNeverWidenTheUsersList(t *testing.T) {
	for _, tc := range []struct {
		user string
		set  bool
		want string
	}{
		{"", false, allowedGitProtocols},
		{"https", true, "https"},
		{"https:ext:http", true, "https"},
		{"ext", true, ""},
	} {
		if got := gitProtocols(tc.user, tc.set); got != tc.want {
			t.Errorf("gitProtocols(%q, %v) = %q, want %q", tc.user, tc.set, got, tc.want)
		}
	}
}
