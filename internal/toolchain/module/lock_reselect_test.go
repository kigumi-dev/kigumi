package module_test

import (
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestPruneLockListsAChangedVersionInsteadOfLockingIt(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	putManifest(t, "example.com/mid", "v1.0.0", "Require { name: \"example.com/dep\", version: \"v1.1.0\" }\n")
	putManifest(t, "example.com/dep", "v1.0.0", "")
	putManifest(t, "example.com/dep", "v1.1.0", "")
	lock := module.Lock{Entries: map[string]module.LockEntry{
		"example.com/mid": {Version: "v1.0.0"}, "example.com/dep": {Version: "v1.1.0"}, "example.com/gone": {Version: "v1.0.0"},
	}}
	kept := module.ModFile{Name: "app", Requires: []module.Require{{Name: "example.com/dep", Version: "v1.0.0"}}}

	next, changes, err := module.PruneLock(testCompiler(), kept, module.Local{}, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Entries) != 0 {
		t.Fatalf("no entry may survive or be created: %+v", next.Entries)
	}
	if len(changes) != 1 || changes[0].String() != "example.com/dep v1.1.0 -> v1.0.0" {
		t.Fatalf("changes = %v", changes)
	}
}

func TestPruneLockKeepsAgreeingEntriesAndNeverCreatesOne(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	putManifest(t, "example.com/dep", "v1.0.0", "")
	putManifest(t, "example.com/other", "v1.0.0", "")
	mf := module.ModFile{Name: "app", Requires: []module.Require{{Name: "example.com/dep", Version: "v1.0.0"}, {Name: "example.com/other", Version: "v1.0.0"}}}
	next, changes, err := module.PruneLock(testCompiler(), mf, module.Local{}, module.Lock{Entries: map[string]module.LockEntry{"example.com/dep": {Version: "v1.0.0", Hash: "h"}}})
	if err != nil || len(changes) != 0 || len(next.Entries) != 1 || next.Entries["example.com/dep"].Hash != "h" {
		t.Fatalf("%+v %v %v", next.Entries, changes, err)
	}
}

func TestLockDisagreementsNamesStaleMissingAndMovedEntries(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	putManifest(t, "example.com/dep", "v1.0.0", "")
	putManifest(t, "example.com/other", "v1.0.0", "")
	putManifest(t, "example.com/moved", "v1.0.0", "")
	mf := module.ModFile{Name: "app", Requires: []module.Require{
		{Name: "example.com/dep", Version: "v1.0.0"}, {Name: "example.com/other", Version: "v1.0.0"},
		{Name: "example.com/moved", Version: "v1.0.0", URL: "file:///new"},
	}}
	lock := module.Lock{Entries: map[string]module.LockEntry{
		"example.com/dep":   {Version: "v1.1.0"},
		"example.com/gone":  {Version: "v1.0.0"},
		"example.com/moved": {Version: "v1.0.0", URL: "file:///old"},
	}}
	problems, err := module.LockDisagreements(testCompiler(), mf, module.Local{}, lock)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(problems, "\n")
	for _, want := range []string{"example.com/dep: the lock has v1.1.0", "example.com/gone v1.0.0: locked but no longer selected", "example.com/other v1.0.0: selected but not in the lock", "example.com/moved v1.0.0: the lock recorded file:///old but the Require now says file:///new"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}
