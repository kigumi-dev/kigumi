package zigtool_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/zigtool"
	"kigumi/internal/zigtool/store"
)

func writeExecutable(path, version string) error {
	return os.WriteFile(path, []byte("#!/bin/sh\nif [ \"$1\" = version ]; then echo '"+version+"'; exit 0; fi\nexit 1\n"), 0o755)
}

func installFake(t *testing.T, root, version string) {
	t.Helper()
	st, err := store.Stage(root, version)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeExecutable(store.BinaryPath(st.Dir), version); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteManifest(st.Dir, store.Manifest{Version: version, InstalledAt: time.Now(), Obtained: "release"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestForModulePrefersStoreOverExplicitAndPATH(t *testing.T) {
	root := t.TempDir()
	installFake(t, root, "0.17.0")
	explicit := fakeZig(t, "0.16.0")

	r := zigtool.ForModule(root, explicit, "0.17.0")
	if r.Source != "managed store" || r.Path != filepath.Join(root, "0.17.0", store.BinaryName()) {
		t.Fatalf("%+v", r)
	}
}

func TestForModuleFallsBackToExplicitWhenStoreLacksThePin(t *testing.T) {
	root := t.TempDir()
	explicit := fakeZig(t, "0.17.0")

	r := zigtool.ForModule(root, explicit, "0.17.0")
	if r.Source != "$KIGUMI_ZIG" || r.Path != explicit || r.Mismatch {
		t.Fatalf("%+v", r)
	}
}

func TestForModuleUnpinnedSkipsStore(t *testing.T) {
	root := t.TempDir()
	installFake(t, root, "0.17.0")
	explicit := fakeZig(t, "9.9.9")

	r := zigtool.ForModule(root, explicit, "")
	if r.Source != "$KIGUMI_ZIG" || r.Path != explicit {
		t.Fatalf("an unpinned module must not be redirected into the store: %+v", r)
	}
}
