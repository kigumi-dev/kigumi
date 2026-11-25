package module_test

// This file pins the behavior of the current single-root resolver as the
// comparison oracle for the revision work (R0.1). The new resolver must keep
// every contract asserted here; the cases reference the acceptance-test IDs
// in docs-new/agent-memo/monorepo_dependency_plan_2026-11-23/revision-validation.md.
// They intentionally avoid duplicating the existing scenario tests
// (modselect_converge_test.go, modselect_url_test.go, alias_test.go,
// modvalid_test.go) and only add contract cases those do not cover.

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

// oracleManifest seeds the fetched cache with one module@version manifest.
type oracleManifest struct {
	name, version, rest string
}

func seedOracleCache(t *testing.T, mods []oracleManifest) {
	t.Helper()
	for _, m := range mods {
		putManifest(t, m.name, m.version, m.rest)
	}
}

// selectOracleCase is one fixed-input selection. want is the exact selected
// Require per effective key; wantBy pins the requirer recorded for it.
type selectOracleCase struct {
	id        string
	manifests []oracleManifest
	root      module.ModFile
	want      map[string]module.Require
	wantBy    map[string]string
	wantErr   string
}

func runSelectOracle(t *testing.T, c selectOracleCase) {
	t.Helper()
	t.Run(c.id, func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		seedOracleCache(t, c.manifests)
		sel, err := module.SelectVersions(testCompiler(), c.root, module.Local{})
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("want error containing %q, got %v (selection %+v)", c.wantErr, err, sel)
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(sel.Requires, c.want) {
			t.Fatalf("selection mismatch:\n got %+v\nwant %+v", sel.Requires, c.want)
		}
		if c.wantBy != nil && !reflect.DeepEqual(sel.By, c.wantBy) {
			t.Fatalf("requirer map mismatch:\n got %+v\nwant %+v", sel.By, c.wantBy)
		}
	})
}

// TestSelectVersionsOracleMerge pins the merge/precedence rules of the
// current resolver (R-T01, R-T03): commit hashes only agree with identical
// hashes, SemVer precedence decides otherwise, and equal-precedence but
// literally different tags conflict instead of silently picking one.
func TestSelectVersionsOracleMerge(t *testing.T) {
	base := "example.com/x"
	for _, c := range []selectOracleCase{
		{
			id:      "R-T01/hash-vs-tag-conflicts",
			root:    module.ModFile{Name: "example.com/app", Requires: []module.Require{{Name: base, Version: "abcdef1"}, {Name: base, Version: "v1.0.0"}}},
			wantErr: "commit hash cannot be reconciled with another version",
		},
		{
			id:      "R-T01/two-hashes-conflict",
			root:    module.ModFile{Name: "example.com/app", Requires: []module.Require{{Name: base, Version: "abcdef1"}, {Name: base, Version: "1234567"}}},
			wantErr: "commit hash cannot be reconciled with another version",
		},
		{
			id:        "R-T01/identical-hash-agrees",
			manifests: []oracleManifest{{name: base, version: "abcdef1"}},
			root: module.ModFile{Name: "example.com/app", Requires: []module.Require{
				{Name: base, Version: "abcdef1"},
				{Name: base, Version: "abcdef1", URL: "https://good.example/x.git"},
			}},
			// mergeRequire fills an empty URL on an identical version.
			want:   map[string]module.Require{base: {Name: base, Version: "abcdef1", URL: "https://good.example/x.git"}},
			wantBy: map[string]string{base: "example.com/app"},
		},
		{
			id:        "R-T03/release-outranks-prerelease",
			manifests: []oracleManifest{{name: base, version: "v1.0.0-rc.1"}, {name: base, version: "v1.0.0"}},
			root: module.ModFile{Name: "example.com/app", Requires: []module.Require{
				{Name: base, Version: "v1.0.0-rc.1"},
				{Name: base, Version: "v1.0.0"},
			}},
			want:   map[string]module.Require{base: {Name: base, Version: "v1.0.0"}},
			wantBy: map[string]string{base: "example.com/app"},
		},
		{
			id:      "R-T03/plain-and-build-tag-conflicts",
			root:    module.ModFile{Name: "example.com/app", Requires: []module.Require{{Name: base, Version: "v1.0.0"}, {Name: base, Version: "v1.0.0+buildA"}}},
			wantErr: "same precedence but different tags",
		},
		{
			id:        "R-T03/identical-build-tag-agrees",
			manifests: []oracleManifest{{name: base, version: "v1.0.0+buildA"}},
			root: module.ModFile{Name: "example.com/app", Requires: []module.Require{
				{Name: base, Version: "v1.0.0+buildA"},
				{Name: base, Version: "v1.0.0+buildA"},
			}},
			want:   map[string]module.Require{base: {Name: base, Version: "v1.0.0+buildA"}},
			wantBy: map[string]string{base: "example.com/app"},
		},
	} {
		runSelectOracle(t, c)
	}
}

// TestSelectVersionsOraclePruning pins the two-pass contract (R-T14, R-T20):
// the maximum per effective key wins regardless of who asked first, the
// winner's own requires survive, and edges reachable only through a
// superseded manifest drop out of the final graph.
func TestSelectVersionsOraclePruning(t *testing.T) {
	runSelectOracle(t, selectOracleCase{
		id: "R-T14/R-T20/winner-requires-survive-loser-edges-do-not",
		manifests: []oracleManifest{
			{name: "example.com/a", version: "v1.0.0", rest: "Require { name: \"example.com/b\", version: \"v1.0.0\" }\nRequire { name: \"example.com/e\", version: \"v1.0.0\" }\n"},
			{name: "example.com/a", version: "v1.1.0", rest: "Require { name: \"example.com/e\", version: \"v1.0.0\" }\n"},
			{name: "example.com/d", version: "v1.0.0", rest: "Require { name: \"example.com/a\", version: \"v1.1.0\" }\n"},
			{name: "example.com/b", version: "v1.0.0"},
			{name: "example.com/e", version: "v1.0.0"},
		},
		root: module.ModFile{Name: "example.com/app", Requires: []module.Require{
			{Name: "example.com/a", Version: "v1.0.0"},
			{Name: "example.com/d", Version: "v1.0.0"},
		}},
		want: map[string]module.Require{
			"example.com/a": {Name: "example.com/a", Version: "v1.1.0"},
			"example.com/d": {Name: "example.com/d", Version: "v1.0.0"},
			"example.com/e": {Name: "example.com/e", Version: "v1.0.0"},
		},
		wantBy: map[string]string{
			"example.com/a": "example.com/d",
			"example.com/d": "example.com/app",
			"example.com/e": "example.com/a",
		},
	})
}

// TestSelectVersionsOracleCycle pins that cycle detection runs on the chosen
// graph only (R-T14): a cycle that exists solely behind a superseded
// manifest must not fail a selection the winner resolved.
func TestSelectVersionsOracleCycle(t *testing.T) {
	for _, c := range []selectOracleCase{
		{
			id:        "R-T14/self-cycle-is-reported",
			manifests: []oracleManifest{{name: "example.com/a", version: "v1.0.0", rest: "Require { name: \"example.com/a\", version: \"v1.0.0\" }\n"}},
			root:      module.ModFile{Name: "example.com/app", Requires: []module.Require{{Name: "example.com/a", Version: "v1.0.0"}}},
			wantErr:   "dependency cycle: example.com/a -> example.com/a",
		},
		{
			id: "R-T14/cycle-only-behind-superseded-manifest-is-ignored",
			manifests: []oracleManifest{
				{name: "example.com/a", version: "v1.0.0", rest: "Require { name: \"example.com/b\", version: \"v1.0.0\" }\n"},
				{name: "example.com/a", version: "v1.1.0"},
				{name: "example.com/b", version: "v1.0.0", rest: "Require { name: \"example.com/a\", version: \"v1.0.0\" }\n"},
				{name: "example.com/c", version: "v1.0.0", rest: "Require { name: \"example.com/a\", version: \"v1.1.0\" }\n"},
			},
			root: module.ModFile{Name: "example.com/app", Requires: []module.Require{
				{Name: "example.com/a", Version: "v1.0.0"},
				{Name: "example.com/c", Version: "v1.0.0"},
			}},
			want: map[string]module.Require{
				"example.com/a": {Name: "example.com/a", Version: "v1.1.0"},
				"example.com/c": {Name: "example.com/c", Version: "v1.0.0"},
			},
			wantBy: map[string]string{
				"example.com/a": "example.com/c",
				"example.com/c": "example.com/app",
			},
		},
	} {
		runSelectOracle(t, c)
	}
}

// TestSelectVersionsOracleAliasURLClaims pins the alias scope of source
// policy (R-T15): URL claims are keyed by effective key, so an alias and a
// plain Require of one real name may name different sources, while the same
// effective key still cannot move its source. Merging stays keyed by alias,
// so aliased versions select independently.
func TestSelectVersionsOracleAliasURLClaims(t *testing.T) {
	const dep = "github.com/acme/x"
	for _, c := range []selectOracleCase{
		{
			id: "R-T15/alias-scopes-url-claims",
			manifests: []oracleManifest{
				{name: dep, version: "v1.0.0"},
				{name: dep, version: "v1.1.0"},
			},
			root: module.ModFile{Name: "example.com/app", Requires: []module.Require{
				{Name: dep, Version: "v1.0.0", Alias: "lib1", URL: "https://one.example/x.git"},
				{Name: dep, Version: "v1.1.0", URL: "https://two.example/x.git"},
			}},
			want: map[string]module.Require{
				"lib1": {Name: dep, Version: "v1.0.0", Alias: "lib1", URL: "https://one.example/x.git"},
				dep:    {Name: dep, Version: "v1.1.0", URL: "https://two.example/x.git"},
			},
			wantBy: map[string]string{"lib1": "example.com/app", dep: "example.com/app"},
		},
		{
			id: "R-T15/same-key-cannot-move-url-at-same-version",
			manifests: []oracleManifest{
				{name: dep, version: "v1.0.0"},
				{name: "example.com/mid", version: "v1.0.0", rest: "Require { name: \"github.com/acme/x\", version: \"v1.0.0\", url: \"https://evil.example/x.git\" }\n"},
			},
			root: module.ModFile{Name: "example.com/app", Requires: []module.Require{
				{Name: dep, Version: "v1.0.0", URL: "https://good.example/x.git"},
				{Name: "example.com/mid", Version: "v1.0.0", URL: "https://good.example/mid.git"},
			}},
			wantErr: "a dependency cannot move where a module comes from",
		},
	} {
		runSelectOracle(t, c)
	}
}

// TestSelectVersionsOracleOrderIndependence pins that selection depends on
// fixed manifests and requests, not on the order root Requires are written
// or map iteration (R-T23). Both orderings must produce the same effective
// keys, versions, aliases, and requirers.
func TestSelectVersionsOracleOrderIndependence(t *testing.T) {
	manifests := []oracleManifest{
		{name: "example.com/p", version: "v1.0.0", rest: "Require { name: \"example.com/m\", version: \"v1.1.0\" }\n"},
		{name: "example.com/q", version: "v1.0.0", rest: "Require { name: \"example.com/r\", version: \"v1.0.0\" }\n"},
		{name: "example.com/r", version: "v1.0.0", rest: "Require { name: \"example.com/m\", version: \"v1.9.0\" }\n"},
		{name: "example.com/m", version: "v1.1.0", rest: "Require { name: \"example.com/z/v3\", version: \"v3.0.0\" }\n"},
		{name: "example.com/m", version: "v1.9.0"},
		{name: "example.com/z/v3", version: "v3.0.0"},
		{name: "example.com/aux", version: "v1.0.0", rest: "Require { name: \"example.com/q\", version: \"v1.0.0\" }\n"},
	}
	want := map[string]module.Require{
		"aux":           {Name: "example.com/aux", Version: "v1.0.0", Alias: "aux"},
		"example.com/m": {Name: "example.com/m", Version: "v1.9.0"},
		"example.com/p": {Name: "example.com/p", Version: "v1.0.0"},
		"example.com/q": {Name: "example.com/q", Version: "v1.0.0"},
		"example.com/r": {Name: "example.com/r", Version: "v1.0.0"},
	}
	wantBy := map[string]string{
		"aux":           "example.com/app",
		"example.com/m": "example.com/r",
		"example.com/p": "example.com/app",
		"example.com/q": "example.com/app",
		"example.com/r": "example.com/q",
	}
	requires := []module.Require{
		{Name: "example.com/p", Version: "v1.0.0"},
		{Name: "example.com/q", Version: "v1.0.0"},
		{Name: "example.com/aux", Version: "v1.0.0", Alias: "aux"},
	}
	for _, order := range permuteOracleRequires(requires) {
		id := "R-T23/root-"
		for _, r := range order {
			id += strings.TrimPrefix(r.Key(), "example.com/") + "-"
		}
		t.Run(id, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			seedOracleCache(t, manifests)
			sel, err := module.SelectVersions(testCompiler(), module.ModFile{Name: "example.com/app", Requires: order}, module.Local{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(sel.Requires, want) || !reflect.DeepEqual(sel.By, wantBy) {
				t.Fatalf("selection changed with root order %v:\n got %+v by %+v\nwant %+v by %+v", oracleRequireKeys(order), sel.Requires, sel.By, want, wantBy)
			}
			if _, ok := sel.Requires["example.com/z/v3"]; ok {
				t.Fatalf("z/v3 is reachable only through the superseded m v1.1.0 and must not survive: %+v", sel.Requires)
			}
		})
	}

	// The order of Require records inside a fixed module's manifest is an
	// input-bytes change, but it must not change what selection means.
	t.Run("R-T23/manifest-require-order", func(t *testing.T) {
		variants := map[string]string{
			"xy": "Require { name: \"example.com/x\", version: \"v1.0.0\" }\nRequire { name: \"example.com/y\", version: \"v1.0.0\" }\n",
			"yx": "Require { name: \"example.com/y\", version: \"v1.0.0\" }\nRequire { name: \"example.com/x\", version: \"v1.0.0\" }\n",
		}
		var first map[string]module.Require
		var firstBy map[string]string
		for _, order := range []string{"xy", "yx"} {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			seedOracleCache(t, []oracleManifest{
				{name: "example.com/base", version: "v1.0.0", rest: variants[order]},
				{name: "example.com/x", version: "v1.0.0", rest: "Require { name: \"example.com/y\", version: \"v1.1.0\" }\n"},
				{name: "example.com/y", version: "v1.0.0"},
				{name: "example.com/y", version: "v1.1.0"},
			})
			sel, err := module.SelectVersions(testCompiler(), module.ModFile{Name: "example.com/app", Requires: []module.Require{{Name: "example.com/base", Version: "v1.0.0"}}}, module.Local{})
			if err != nil {
				t.Fatal(err)
			}
			if sel.Requires["example.com/y"].Version != "v1.1.0" {
				t.Fatalf("%s: y should converge to v1.1.0: %+v", order, sel.Requires)
			}
			if first == nil {
				first, firstBy = sel.Requires, sel.By
				continue
			}
			if !reflect.DeepEqual(sel.Requires, first) || !reflect.DeepEqual(sel.By, firstBy) {
				t.Fatalf("manifest require order changed selection:\n xy %+v by %+v\n yx %+v by %+v", first, firstBy, sel.Requires, sel.By)
			}
		}
	})
}

func permuteOracleRequires(in []module.Require) [][]module.Require {
	if len(in) <= 1 {
		return [][]module.Require{append([]module.Require(nil), in...)}
	}
	var out [][]module.Require
	for i := range in {
		rest := make([]module.Require, 0, len(in)-1)
		rest = append(rest, in[:i]...)
		rest = append(rest, in[i+1:]...)
		for _, tail := range permuteOracleRequires(rest) {
			out = append(out, append([]module.Require{in[i]}, tail...))
		}
	}
	return out
}

func oracleRequireKeys(rs []module.Require) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Key()
	}
	return out
}

// TestLocalReplacementAliasPrecedence pins R-T22's local Replace contract at
// the API level: an alias-specific Replace wins over a plain name Replace,
// and the plain name entry still covers aliased Requires that have no
// specific entry.
func TestLocalReplacementAliasPrecedence(t *testing.T) {
	local := module.Local{Replaces: map[string]string{
		"example.com/lib": "/plain",
		"lib1":            "/alias1",
	}}
	for _, c := range []struct {
		name string
		r    module.Require
		want string
		ok   bool
	}{
		{"alias-specific-wins", module.Require{Name: "example.com/lib", Version: "v1.0.0", Alias: "lib1"}, "/alias1", true},
		{"plain-covers-other-alias", module.Require{Name: "example.com/lib", Version: "v1.0.0", Alias: "lib2"}, "/plain", true},
		{"plain-covers-no-alias", module.Require{Name: "example.com/lib", Version: "v1.0.0"}, "/plain", true},
		{"unrelated-name", module.Require{Name: "example.com/other", Version: "v1.0.0"}, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := local.Replacement(c.r)
			if got != c.want || ok != c.ok {
				t.Fatalf("Replacement(%+v) = %q, %v; want %q, %v", c.r, got, ok, c.want, c.ok)
			}
		})
	}
}

// TestSelectVersionsOracleLocalReplace pins R-T22 in selection: a replaced
// module is read from its local directory instead of the cache, keeps the
// Require's version, and its manifest's own Requires join the search.
// Alias-specific and plain Replaces apply to separate effective keys.
func TestSelectVersionsOracleLocalReplace(t *testing.T) {
	t.Run("local-overrides-cache-and-keeps-requested-version", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		root := t.TempDir()
		localDep := filepath.Join(root, "dep")
		writeModule(t, localDep, manifestFor("example.com/dep", "example.com/sub", "v1.0.0"))
		writeModule(t, filepath.Join(root, "sub"), manifestFor("example.com/sub"))
		// The cache copy must not be consulted while a Replace exists.
		putManifest(t, "example.com/dep", "v1.0.0", "Require { name: \"example.com/cacheonly\", version: \"v1.0.0\" }\n")
		putManifest(t, "example.com/cacheonly", "v1.0.0", "")

		local := module.Local{Replaces: map[string]string{
			"example.com/dep": localDep,
			"example.com/sub": filepath.Join(root, "sub"),
		}}
		sel, err := module.SelectVersions(testCompiler(), module.ModFile{Name: "example.com/app", Requires: []module.Require{
			{Name: "example.com/dep", Version: "v1.0.0"},
		}}, local)
		if err != nil {
			t.Fatal(err)
		}
		if got := sel.Requires["example.com/dep"]; got != (module.Require{Name: "example.com/dep", Version: "v1.0.0"}) {
			t.Fatalf("local Replace must keep the requested version: %+v", got)
		}
		if got := sel.Requires["example.com/sub"]; got.Version != "v1.0.0" {
			t.Fatalf("requires of the local manifest should join the search: %+v", sel.Requires)
		}
		if _, ok := sel.Requires["example.com/cacheonly"]; ok {
			t.Fatalf("the cache manifest must not be read when a Replace exists: %+v", sel.Requires)
		}
	})

	t.Run("alias-specific-and-plain-replaces-split-by-key", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		root := t.TempDir()
		aliasDir := filepath.Join(root, "alias1")
		plainDir := filepath.Join(root, "plain")
		writeModule(t, aliasDir, manifestFor("example.com/lib", "example.com/fromalias", "v1.0.0"))
		writeModule(t, plainDir, manifestFor("example.com/lib", "example.com/fromplain", "v1.0.0"))
		writeModule(t, filepath.Join(root, "fromalias"), manifestFor("example.com/fromalias"))
		writeModule(t, filepath.Join(root, "fromplain"), manifestFor("example.com/fromplain"))

		local := module.Local{Replaces: map[string]string{
			"lib1":                  aliasDir,
			"example.com/lib":       plainDir,
			"example.com/fromalias": filepath.Join(root, "fromalias"),
			"example.com/fromplain": filepath.Join(root, "fromplain"),
		}}
		sel, err := module.SelectVersions(testCompiler(), module.ModFile{Name: "example.com/app", Requires: []module.Require{
			{Name: "example.com/lib", Version: "v1.0.0", Alias: "lib1"},
			{Name: "example.com/lib", Version: "v1.1.0", Alias: "lib2"},
		}}, local)
		if err != nil {
			t.Fatal(err)
		}
		for key, version := range map[string]string{"lib1": "v1.0.0", "lib2": "v1.1.0"} {
			if got := sel.Requires[key]; got.Version != version || got.Name != "example.com/lib" {
				t.Fatalf("%s: %+v", key, got)
			}
		}
		for _, key := range []string{"example.com/fromalias", "example.com/fromplain"} {
			if _, ok := sel.Requires[key]; !ok {
				t.Fatalf("%s should be selected from the replaced manifests: %+v", key, sel.Requires)
			}
		}
	})

	t.Run("local-manifest-name-mismatch-is-rejected", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		dep := filepath.Join(t.TempDir(), "dep")
		writeModule(t, dep, manifestFor("example.com/other"))
		local := module.Local{Replaces: map[string]string{"example.com/dep": dep}}
		_, err := module.SelectVersions(testCompiler(), module.ModFile{Name: "example.com/app", Requires: []module.Require{
			{Name: "example.com/dep", Version: "v1.0.0"},
		}}, local)
		if err == nil || !strings.Contains(err.Error(), `names the module "example.com/other"`) {
			t.Fatalf("want the local manifest name mismatch, got %v", err)
		}
	})
}
