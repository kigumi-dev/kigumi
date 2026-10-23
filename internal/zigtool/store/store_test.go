package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/zigtool/store"
)

func TestRootOverrideWins(t *testing.T) {
	got, err := store.Root("/some/dir", "")
	if err != nil || got != "/some/dir" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRootDefaultsUnderHome(t *testing.T) {
	home := t.TempDir()
	got, err := store.Root("", home)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", "kigumi", "zig"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func stageAndCommit(t *testing.T, root, version string) {
	t.Helper()
	st, err := store.Stage(root, version)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.BinaryPath(st.Dir), []byte("#!/bin/sh\necho "+version+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := store.Manifest{Version: version, OS: "linux", Arch: "x86_64", InstalledAt: time.Now(), Obtained: "release"}
	if err := store.WriteManifest(st.Dir, m); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestStageCommitListRemove(t *testing.T) {
	root := t.TempDir()
	stageAndCommit(t, root, "0.17.0")

	path, ok := store.Installed(root, "0.17.0")
	if !ok || path != store.BinaryPath(store.VersionDir(root, "0.17.0")) {
		t.Fatalf("Installed = %q, %v", path, ok)
	}
	m, err := store.ReadManifest(store.VersionDir(root, "0.17.0"))
	if err != nil || m.Version != "0.17.0" || m.Obtained != "release" {
		t.Fatalf("manifest = %+v, err = %v", m, err)
	}
	versions, err := store.List(root)
	if err != nil || len(versions) != 1 || versions[0] != "0.17.0" {
		t.Fatalf("List = %v, err = %v", versions, err)
	}
	if err := store.Remove(root, "0.17.0"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Installed(root, "0.17.0"); ok {
		t.Fatal("still installed after Remove")
	}
	if err := store.Remove(root, "0.17.0"); err != nil {
		t.Fatalf("Remove of an absent version must not error: %v", err)
	}
}

func TestCommitReplacesExisting(t *testing.T) {
	root := t.TempDir()
	stageAndCommit(t, root, "0.17.0")
	stageAndCommit(t, root, "0.17.0")
	versions, err := store.List(root)
	if err != nil || len(versions) != 1 {
		t.Fatalf("List = %v, err = %v (re-staging must replace, not duplicate)", versions, err)
	}
}

func TestListSkipsUnfinishedStaging(t *testing.T) {
	root := t.TempDir()
	st, err := store.Stage(root, "0.17.0")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Discard()
	versions, err := store.List(root)
	if err != nil || len(versions) != 0 {
		t.Fatalf("List must not see an uncommitted stage: %v, err=%v", versions, err)
	}
}

func TestListSkipsStageWithBinary(t *testing.T) {
	root := t.TempDir()
	st, err := store.Stage(root, "0.17.0")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Discard()
	if err := os.WriteFile(store.BinaryPath(st.Dir), []byte("#!/bin/sh\necho 0.17.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	versions, err := store.List(root)
	if err != nil || len(versions) != 0 {
		t.Fatalf("List must not report an interrupted install: %v, err=%v", versions, err)
	}
}

func TestListOnMissingRootIsEmpty(t *testing.T) {
	versions, err := store.List(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil || versions != nil {
		t.Fatalf("got %v, %v", versions, err)
	}
}

func TestRemoveRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	victim := t.TempDir()
	if err := os.WriteFile(filepath.Join(victim, "precious.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, victim)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Remove(root, rel); err == nil {
		t.Fatal("expected an error for a version string containing a path separator")
	}
	if _, err := os.Stat(filepath.Join(victim, "precious.txt")); err != nil {
		t.Fatalf("victim must survive: %v", err)
	}
}

func TestInstalledRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	if path, ok := store.Installed(root, "../etc"); ok || path != "" {
		t.Fatalf("Installed(%q) = %q, %v, want \"\", false", "../etc", path, ok)
	}
}

func TestStageRejectsInvalidVersion(t *testing.T) {
	root := t.TempDir()
	for _, v := range []string{"", ".", "..", "../x", "a/b"} {
		if _, err := store.Stage(root, v); err == nil {
			t.Errorf("Stage(%q) succeeded, want an error", v)
		}
	}
}
