package module_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestVersionRequestDistinguishesExactFromRange(t *testing.T) {
	for _, v := range []string{"v1.2.3", "v1.2.3-rc.1", "v1.2.3+meta", "0123456abcdef"} {
		rng, err := module.ParseVersionRequest(v)
		if err != nil || rng != nil {
			t.Errorf("ParseVersionRequest(%q) = %v, %v; want exact", v, rng, err)
		}
	}
	rng, err := module.ParseVersionRequest("^1.2.3")
	if err != nil || rng == nil {
		t.Fatalf("ParseVersionRequest range = %v, %v", rng, err)
	}
	if err := module.ValidateVersionRequest("not a version"); err == nil {
		t.Fatal("invalid request accepted")
	}
}

func TestVersionRangeAccepts(t *testing.T) {
	cases := []struct {
		request string
		accept  []string
		reject  []string
	}{
		{"^1.2.3", []string{"v1.2.3", "v1.9.9"}, []string{"v1.2.2", "v2.0.0"}},
		{"^0.2.3", []string{"v0.2.3", "v0.2.9"}, []string{"v0.3.0"}},
		{"^0.0.3", []string{"v0.0.3"}, []string{"v0.0.4"}},
		{"~1.2.3", []string{"v1.2.3", "v1.2.9"}, []string{"v1.3.0"}},
		{">=1.2.0 <2.0.0", []string{"v1.2.0", "v1.9.0"}, []string{"v1.1.9", "v2.0.0"}},
		{"^ 1.0.0", []string{"v1.0.0", "v1.2.1"}, []string{"v2.0.0"}},
		{"~ 1.0.0", []string{"v1.0.1"}, []string{"v1.1.0"}},
		{">= 1.0.0 < 1.2.0", []string{"v1.1.0"}, []string{"v1.2.0"}},
		{"< 1.2.0", []string{"v1.1.0"}, []string{"v1.2.0"}},
		{"<=1.2", []string{"v1.2.5"}, []string{"v1.3.0"}},
		{"=1.2", []string{"v1.2.5"}, []string{"v1.3.0"}},
		{">1.2", []string{"v1.3.0"}, []string{"v1.2.5"}},
		{">1", []string{"v2.0.0"}, []string{"v1.9.0"}},
		{"1.2.x", []string{"v1.2.0", "v1.2.9"}, []string{"v1.3.0", "v1.1.9"}},
		{"v1.2.3 || >=2.0.0", []string{"v1.2.3", "v2.1.0"}, []string{"v1.2.4"}},
		{"*", []string{"v0.0.1", "v9.9.9"}, nil},
	}
	for _, tc := range cases {
		rng, err := module.ParseVersionRange(tc.request)
		if err != nil {
			t.Fatalf("ParseVersionRange(%q): %v", tc.request, err)
		}
		for _, v := range tc.accept {
			if !rng.AcceptsString(v) {
				t.Errorf("%q should accept %s", tc.request, v)
			}
		}
		for _, v := range tc.reject {
			if rng.AcceptsString(v) {
				t.Errorf("%q should reject %s", tc.request, v)
			}
		}
	}
}

func TestVersionRangePrereleaseOptIn(t *testing.T) {
	stable, err := module.ParseVersionRange(">=1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if stable.AcceptsString("v1.1.0-rc.1") {
		t.Fatal("a stable range accepted a prerelease")
	}
	pre, err := module.ParseVersionRange(">=1.1.0-rc.1")
	if err != nil {
		t.Fatal(err)
	}
	if !pre.AcceptsString("v1.1.0-rc.1") {
		t.Fatal("a prerelease range rejected its prerelease")
	}
	if pre.AcceptsString("v1.6.0-beta") {
		t.Fatal("a prerelease range accepted a different tuple's prerelease")
	}
	union, err := module.ParseVersionRange(">=1.0.0 || >=2.0.0-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if union.AcceptsString("v1.5.0-rc.1") || union.AcceptsString("v2.1.0-beta") {
		t.Fatal("a prerelease leaked into an alternative without its tuple")
	}
	upper, err := module.ParseVersionRange(">=1.0.0 <2.0.0-0")
	if err != nil {
		t.Fatal(err)
	}
	if upper.AcceptsString("v1.5.0-rc.1") {
		t.Fatal("a prerelease upper bound enabled prereleases for other tuples")
	}
}

func TestVersionRangeRejectsMalformed(t *testing.T) {
	for _, v := range []string{"", "^", ">=1.2.3-", "^1.2.3 ||", "1.x.y", "^v1.2.3-"} {
		if _, err := module.ParseVersionRange(v); err == nil {
			t.Errorf("ParseVersionRange(%q) accepted malformed input", v)
		}
	}
}

func TestRequireRangeTracksSelectedVersion(t *testing.T) {
	rng, err := module.ParseVersionRange("^1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	r := module.Require{Name: "example.com/dep", Version: "^1.2.3", Range: rng}
	selected := r.WithVersion("v1.4.0")
	if selected.IsRange() || selected.Range == nil || selected.Version != "v1.4.0" || !selected.Accepts("v1.4.0") {
		t.Fatalf("selected range request = %+v", selected)
	}
	if selected.Accepts("v2.0.0") {
		t.Fatal("selected request accepted a version outside its range")
	}
	if !strings.Contains(selected.Range.String(), "1.2.3") {
		t.Fatalf("range text lost: %q", selected.Range.String())
	}
}

func TestSelectVersionsResolvesRange(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	putManifest(t, "dep", "v1.0.0", "")
	putManifest(t, "dep", "v1.2.0", "")
	putManifest(t, "dep", "v1.4.0", "")

	rng, err := module.ParseVersionRange("^1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	root := module.ModFile{Requires: []module.Require{{Name: "dep", Version: "^1.2.0", Range: rng}}}
	resolve := func(r module.Require) (module.Require, error) {
		return r.WithVersion("v1.4.0"), nil
	}
	sel, err := module.SelectVersionsResolved(testCompiler(), root, module.Local{}, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if got := sel.Requires["dep"].Version; got != "v1.4.0" {
		t.Fatalf("selected %s, want v1.4.0", got)
	}

	if _, err := module.SelectVersions(testCompiler(), root, module.Local{}); err == nil || !strings.Contains(err.Error(), "semver range") {
		t.Fatalf("unresolved range error = %v", err)
	}
	bad := func(r module.Require) (module.Require, error) { return r.WithVersion("v2.0.0"), nil }
	if _, err := module.SelectVersionsResolved(testCompiler(), root, module.Local{}, bad); err == nil {
		t.Fatal("range accepted a selected version outside its bounds")
	}
}

func TestLockResolverUsesLockedVersion(t *testing.T) {
	rng, err := module.ParseVersionRange("^1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	req := module.Require{Name: "dep", Version: "^1.2.0", Range: rng}
	lock := module.Lock{Entries: map[string]module.LockEntry{
		"dep": {Name: "dep", Version: "v1.4.0"},
	}}
	got, err := module.LockResolver(lock)(req)
	if err != nil || got.Version != "v1.4.0" || got.Range == nil || got.IsRange() {
		t.Fatalf("LockResolver = %+v, %v", got, err)
	}

	outside := module.Lock{Entries: map[string]module.LockEntry{
		"dep": {Name: "dep", Version: "v2.0.0"},
	}}
	if _, err := module.LockResolver(outside)(req); err == nil {
		t.Fatal("LockResolver accepted a version outside the range")
	}
}

func TestManifestAcceptsRangeButRejectsMalformed(t *testing.T) {
	root := t.TempDir()
	src := "Module { name: \"app\", kigumi: \"0.1\" }\nRequire { name: \"example.com/dep\", version: \"^1.2.0\" }\n"
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	mf, ok, err := module.ReadModFile(root)
	if err != nil || !ok {
		t.Fatalf("ReadModFile = %v, %v", ok, err)
	}
	if len(mf.Requires) != 1 || !mf.Requires[0].IsRange() || mf.Requires[0].Version != "^1.2.0" {
		t.Fatalf("range lost: %+v", mf.Requires)
	}
	bad := "Module { name: \"app\", kigumi: \"0.1\" }\nRequire { name: \"example.com/dep\", version: \"^\" }\n"
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := module.ReadModFile(root); err == nil {
		t.Fatal("malformed range accepted")
	}
}

func TestRangeLockAgreesWithTidyAndVerify(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	putManifest(t, "dep", "v1.4.0", "")
	rng, err := module.ParseVersionRange("^1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	root := module.ModFile{Name: "app", Requires: []module.Require{{Name: "dep", Version: "^1.2.0", Range: rng}}}
	lock := module.Lock{Entries: map[string]module.LockEntry{
		"dep": {Name: "dep", Version: "v1.4.0"},
	}}
	problems, err := module.LockDisagreements(testCompiler(), root, module.Local{}, lock)
	if err != nil || len(problems) != 0 {
		t.Fatalf("LockDisagreements = %v, %v", problems, err)
	}
	next, changes, err := module.PruneLock(testCompiler(), root, module.Local{}, lock)
	if err != nil || len(changes) != 0 || next.Entries["dep"].Version != "v1.4.0" {
		t.Fatalf("PruneLock = %+v, %v, %v", next, changes, err)
	}
}
