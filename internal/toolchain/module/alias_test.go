package module_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/module"
)

// TestParseRequireAlias covers the manifest and script syntax, the effective
// key, and the alias diagnostics.
func TestParseRequireAlias(t *testing.T) {
	src := "Module { name: \"github.com/acme/app\", kigumi: \"0.1\" }\n" +
		"Require { name: \"github.com/acme/lib\", version: \"v1.0.0\", alias: \"lib1\" }\n" +
		"Require { name: \"github.com/acme/lib\", version: \"v1.1.0\", alias: \"lib2\" }\n"
	mf, ds := module.ParseModFile(token.NewFile("mod.kg", []byte(src)))
	if len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", ds)
	}
	if len(mf.Requires) != 2 || mf.Requires[0].Alias != "lib1" || mf.Requires[1].Alias != "lib2" {
		t.Fatalf("parsed %+v", mf.Requires)
	}
	if mf.Requires[0].Key() != "lib1" || mf.Requires[1].Key() != "lib2" {
		t.Fatalf("effective keys: %q %q", mf.Requires[0].Key(), mf.Requires[1].Key())
	}

	script := "Kigumi { version: \"0.1\" }\n" +
		"Require { name: \"github.com/acme/lib\", version: \"v1.0.0\", alias: \"lib1\" }\n" +
		"print \"hi\"\n"
	smf, has, err := module.ReadScriptManifest(writeScript(t, script))
	if err != nil || !has || len(smf.Requires) != 1 || smf.Requires[0].Alias != "lib1" {
		t.Fatalf("script manifest: %+v has=%v err=%v", smf, has, err)
	}

	bad := map[string]string{
		"Require { name: \"x\", version: \"v1.0.0\", alias: \"bad alias\" }\n":                                                             "not a valid import prefix",
		"Require { name: \"x\", version: \"v1.0.0\", alias: \"std\" }\n":                                                                   "reserved for the standard library",
		"Require { name: \"x\", version: \"v1.0.0\", alias: \"lib\" }\nRequire { name: \"y\", version: \"v1.0.0\", alias: \"lib\" }\n":     "used by two Require records",
		"Require { name: \"x\", version: \"v1.0.0\", alias: \"lib\" }\nRequire { name: \"lib\", version: \"v1.0.0\" }\n":                   "collides with the module",
		"Require { name: \"x\", version: \"v1.0.0\", alias: \"lib\" }\nRequire { name: \"y\", version: \"v1.0.0\", alias: \"lib/sub\" }\n": "overlaps alias",
		"Require { name: \"x\", version: \"v1.0.0\", alias: \"github.com/acme/app\" }\n":                                                   "overlaps this module's name",
	}
	for rest, want := range bad {
		_, ds := module.ParseModFile(token.NewFile("mod.kg", manifest("github.com/acme/app", rest)))
		found := false
		for _, d := range ds {
			found = found || strings.Contains(d.Msg, want)
		}
		if !found {
			t.Errorf("%q: want %q, got %+v", rest, want, ds)
		}
	}
}

func writeScript(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tool.kg")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAppendRequireAliasRoundTrip verifies mod.kg and script writers preserve
// aliases and allow two aliases of one real module.
func TestAppendRequireAliasRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := module.WriteManifest(root, "github.com/acme/app"); err != nil {
		t.Fatal(err)
	}
	lib := module.Require{Name: "github.com/acme/lib", Version: "v1.0.0", Alias: "lib1"}
	if err := module.AppendRequire(root, lib); err != nil {
		t.Fatal(err)
	}
	if err := module.AppendRequire(root, module.Require{Name: lib.Name, Version: "v1.1.0", Alias: "lib2"}); err != nil {
		t.Fatalf("a second alias of the same module should be allowed: %v", err)
	}
	if err := module.AppendRequire(root, lib); err == nil {
		t.Fatal("a duplicate effective key should be refused")
	}
	mf, ok, err := module.ReadModFile(root)
	if err != nil || !ok || len(mf.Requires) != 2 || mf.Requires[0].Alias != "lib1" || mf.Requires[1].Alias != "lib2" {
		t.Fatalf("round trip: %+v %v %v", mf, ok, err)
	}

	script := filepath.Join(root, "tool.kg")
	os.WriteFile(script, []byte("Kigumi { version: \"0.1\" }\nprint \"hi\"\n"), 0o644)
	if err := module.AppendScriptRequire(script, lib); err != nil {
		t.Fatal(err)
	}
	smf, has, err := module.ReadScriptManifest(script)
	if err != nil || !has || len(smf.Requires) != 1 || smf.Requires[0].Alias != "lib1" {
		t.Fatalf("script round trip: %+v %v %v", smf, has, err)
	}
}

// TestSelectVersionsAliases checks that two aliases of one real name select
// independently while a plain Require keeps the old MVS behavior.
func TestSelectVersionsAliases(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	putManifest(t, "github.com/acme/lib", "v1.0.0", "")
	putManifest(t, "github.com/acme/lib", "v1.1.0", "")

	root := module.ModFile{Name: "app", Requires: []module.Require{
		{Name: "github.com/acme/lib", Version: "v1.0.0", Alias: "lib1"},
		{Name: "github.com/acme/lib", Version: "v1.1.0", Alias: "lib2"},
	}}
	sel, err := module.SelectVersions(testCompiler(), root, module.Local{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Requires) != 2 || sel.Requires["lib1"].Version != "v1.0.0" || sel.Requires["lib2"].Version != "v1.1.0" {
		t.Fatalf("aliases did not select independently: %+v", sel.Requires)
	}
	if sel.Requires["lib1"].Name != "github.com/acme/lib" || sel.Requires["lib2"].Name != "github.com/acme/lib" {
		t.Fatalf("real names lost: %+v", sel.Requires)
	}

	plain := module.ModFile{Name: "app", Requires: []module.Require{
		{Name: "github.com/acme/lib", Version: "v1.0.0"},
		{Name: "github.com/acme/lib", Version: "v1.1.0"},
	}}
	sel, err = module.SelectVersions(testCompiler(), plain, module.Local{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Requires) != 1 || sel.Requires["github.com/acme/lib"].Version != "v1.1.0" {
		t.Fatalf("plain MVS behavior changed: %+v", sel.Requires)
	}

	// One key cannot name two different real modules.
	conflict := module.ModFile{Name: "app", Requires: []module.Require{
		{Name: "github.com/acme/a", Version: "v1.0.0", Alias: "shared"},
		{Name: "github.com/acme/b", Version: "v1.0.0", Alias: "shared"},
	}}
	if _, err := module.SelectVersions(testCompiler(), conflict, module.Local{}); err == nil || !strings.Contains(err.Error(), "names both") {
		t.Fatalf("cross-module alias collision not reported: %v", err)
	}

	// Two aliases of one version cannot come from different repos: the cache
	// entry is keyed by real name and version only.
	urlConflict := module.ModFile{Name: "app", Requires: []module.Require{
		{Name: "github.com/acme/lib", Version: "v1.0.0", Alias: "lib1", URL: "https://example.invalid/a.git"},
		{Name: "github.com/acme/lib", Version: "v1.0.0", Alias: "lib2", URL: "https://example.invalid/b.git"},
	}}
	if _, err := module.SelectVersions(testCompiler(), urlConflict, module.Local{}); err == nil || !strings.Contains(err.Error(), "different sources") {
		t.Fatalf("alias source conflict not reported: %v", err)
	}

	// An alias is an import prefix and may not overlap the root's own.
	rootAlias := module.ModFile{Name: "app", Requires: []module.Require{
		{Name: "github.com/acme/lib", Version: "v1.0.0", Alias: "app/sub"},
	}}
	if _, err := module.SelectVersions(testCompiler(), rootAlias, module.Local{}); err == nil || !strings.Contains(err.Error(), "overlaps the root module") {
		t.Fatalf("root alias overlap not reported: %v", err)
	}
}

// TestRenamePreservesAlias checks that renaming a module's real path keeps
// the alias and does not rewrite imports that use it.
func TestRenamePreservesAlias(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module { name: \"app\", kigumi: \"0.1\" }\n"+
		"Require { name: \"github.com/acme/lib\", version: \"v1.0.0\", alias: \"lib1\" }\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import {one} from lib1\nprint \"${one()}\"\n"), 0o644)
	if _, _, err := module.RenameModule(root, "github.com/acme/lib", "github.com/acme/lib2"); err != nil {
		t.Fatal(err)
	}
	mf, ok, err := module.ReadModFile(root)
	if err != nil || !ok || len(mf.Requires) != 1 || mf.Requires[0].Name != "github.com/acme/lib2" || mf.Requires[0].Alias != "lib1" {
		t.Fatalf("rename changed the alias: %+v %v %v", mf, ok, err)
	}
	src, _ := os.ReadFile(filepath.Join(root, "main.kg"))
	if !strings.Contains(string(src), "from lib1") || strings.Contains(string(src), "lib2") {
		t.Fatalf("alias import should not be rewritten:\n%s", src)
	}
}

// TestLockAliasRoundTripAndDisagreements covers alias fields in mod.lock.kg
// and verification against an aliased manifest.
func TestLockAliasRoundTripAndDisagreements(t *testing.T) {
	root := t.TempDir()
	identity, err := module.NewSourceIdentity("github.com/acme/lib", "https://github.com/acme/lib", "")
	if err != nil {
		t.Fatal(err)
	}
	sourceID := module.LockSourceID(identity)
	commit := mustCommit(t, lockCommit('a'))
	tm := lockTime(t)
	binding := func(alias, version string) (module.LockResolution, module.LockBinding) {
		cv := mustCanonical(t, version)
		res := module.LockResolution{
			ID:        module.LockResolutionID(sourceID, version),
			SourceID:  sourceID,
			Requested: version,
			Version:   cv,
			Commit:    commit,
			Time:      tm,
			Manifest:  lockDigest('2'),
			Tag:       version,
		}
		return res, module.LockBinding{
			Name: identity.Name, Alias: alias, SourceID: sourceID, ResolutionID: res.ID,
			Version: cv, Commit: commit, Hash: lockDigest('3'), URL: identity.URL,
		}
	}
	res1, lock1 := binding("lib1", "v1.0.0")
	res2, lock2 := binding("lib2", "v1.1.0")
	state := module.LockState{
		Kind:        module.LockKindModule,
		Format:      module.LockFormat1,
		Resolver:    module.LockResolverOrderedV1,
		Inputs:      lockDigest('1'),
		Sources:     []module.LockSource{{ID: sourceID, Identity: identity}},
		Resolutions: []module.LockResolution{res1, res2},
		Locks:       []module.LockBinding{lock1, lock2},
	}
	if err := module.WriteLockState(root, state); err != nil {
		t.Fatal(err)
	}
	got, err := module.ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	want1 := module.LockEntry{Name: identity.Name, Alias: "lib1", Version: "v1.0.0", Commit: commit.String(), Hash: lockDigest('3'), URL: identity.URL}
	want2 := module.LockEntry{Name: identity.Name, Alias: "lib2", Version: "v1.1.0", Commit: commit.String(), Hash: lockDigest('3'), URL: identity.URL}
	if got.Entries["lib1"] != want1 || got.Entries["lib2"] != want2 {
		t.Fatalf("alias entries did not round trip: %+v", got.Entries)
	}
	if r := got.Entries["lib1"].Require(); r.Name != identity.Name || r.Alias != "lib1" || r.Version != "v1.0.0" {
		t.Fatalf("Require() = %+v", r)
	}

	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	putManifest(t, "github.com/acme/lib", "v1.0.0", "")
	putManifest(t, "github.com/acme/lib", "v1.1.0", "")
	mf := module.ModFile{Name: "app", Requires: []module.Require{
		{Name: "github.com/acme/lib", Version: "v1.0.0", Alias: "lib1"},
		{Name: "github.com/acme/lib", Version: "v1.1.0", Alias: "lib2"},
	}}
	lock := module.Lock{Entries: map[string]module.LockEntry{
		"lib1": {Name: "github.com/acme/lib", Alias: "lib1", Version: "v1.0.0"},
		"lib2": {Name: "github.com/acme/lib", Alias: "lib2", Version: "v1.1.0"},
	}}
	problems, err := module.LockDisagreements(testCompiler(), mf, module.Local{}, lock)
	if err != nil || len(problems) != 0 {
		t.Fatalf("matching alias lock should verify: %v %+v", err, problems)
	}
	lock.Entries["lib2"] = module.LockEntry{Name: "github.com/acme/lib", Alias: "lib2", Version: "v1.0.0"}
	problems, err = module.LockDisagreements(testCompiler(), mf, module.Local{}, lock)
	if err != nil || len(problems) != 1 || !strings.Contains(problems[0], "lib2") || !strings.Contains(problems[0], "v1.1.0") {
		t.Fatalf("alias version disagreement not reported: %v %+v", err, problems)
	}
	lock.Entries["lib2"] = module.LockEntry{Name: "github.com/acme/other", Alias: "lib2", Version: "v1.1.0"}
	problems, err = module.LockDisagreements(testCompiler(), mf, module.Local{}, lock)
	if err != nil || len(problems) != 1 || !strings.Contains(problems[0], "names github.com/acme/other") {
		t.Fatalf("alias name disagreement not reported: %v %+v", err, problems)
	}
}
