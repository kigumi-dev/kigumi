package module

import (
	"strings"
	"testing"
)

// A /vN suffix is the major version in the module's identity; v0 and v1
// paths carry none, and a version-shaped last segment is a mistake.
func TestPathMajor(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"example.com/mod/v2", "example.com/mono/sgx/v10", "example.com/mod/v2/extra", "v2"} {
		want := ""
		switch name {
		case "example.com/mod/v2":
			want = "/v2"
		case "example.com/mono/sgx/v10":
			want = "/v10"
		}
		got, err := PathMajor(name)
		if err != nil || got != want {
			t.Errorf("PathMajor(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"example.com/mod/v1", "example.com/mod/v0", "example.com/mod/v01", "example.com/mod/v2.0"} {
		if _, err := PathMajor(name); err == nil {
			t.Errorf("PathMajor(%q) must reject an invalid suffix", name)
		}
		if err := ValidModuleName(name); err == nil {
			t.Errorf("ValidModuleName(%q) must reject an invalid suffix", name)
		}
	}
}

// A v2 module at the repository root tags as v2.0.0; a nested v2 module
// lives in <subdir>/v2 and tags as <subdir>/v2/v2.0.0.
func TestRepoOfMajorSuffix(t *testing.T) {
	t.Parallel()
	url, sub, ok := RepoOf(Require{Name: "github.com/go-chi/chi/v5"})
	if !ok || url != "https://github.com/go-chi/chi.git" || sub != "" {
		t.Fatalf("root v2 module: %q %q %v", url, sub, ok)
	}
	url, sub, ok = RepoOf(Require{Name: "github.com/acme/mono/sgx/v2"})
	if !ok || url != "https://github.com/acme/mono.git" || sub != "sgx/v2" {
		t.Fatalf("nested v2 module: %q %q %v", url, sub, ok)
	}
	url, sub, ok = RepoOf(Require{Name: "git.example.com/team/repo/v2", URL: "ssh://git@git.example.com/team/repo.git"})
	if !ok || sub != "" {
		t.Fatalf("explicit url root v2 module: %q %q %v", url, sub, ok)
	}
	if TagOf(sub, "v2.0.0") != "v2.0.0" {
		t.Fatalf("root module tag: %q", TagOf(sub, "v2.0.0"))
	}
	if got := TagOf("sgx/v2", "v2.0.0"); got != "sgx/v2/v2.0.0" {
		t.Fatalf("nested module tag: %q", got)
	}
}

func TestCheckVersionForName(t *testing.T) {
	t.Parallel()
	good := [][2]string{
		{"example.com/mod", "v0.9.0"},
		{"example.com/mod", "v1.0.0"},
		{"example.com/mod/v2", "v2.0.0"},
		{"example.com/mod/v2", "v2.3.1-rc.1"},
		{"example.com/mod/v2", "0123456abcdef"},
	}
	for _, c := range good {
		if err := CheckVersionForName(c[0], c[1]); err != nil {
			t.Errorf("%s %s should be valid: %v", c[0], c[1], err)
		}
	}
	bad := [][2]string{
		{"example.com/mod", "v2.0.0"},
		{"example.com/mod/v2", "v1.0.0"},
		{"example.com/mod/v2", "v3.0.0"},
	}
	for _, c := range bad {
		err := CheckVersionForName(c[0], c[1])
		if err == nil {
			t.Errorf("%s %s should be refused", c[0], c[1])
			continue
		}
		if c[0] == "example.com/mod" && !strings.Contains(err.Error(), "example.com/mod/v2") {
			t.Errorf("the error should name the suffixed path to use, got %v", err)
		}
	}
}
