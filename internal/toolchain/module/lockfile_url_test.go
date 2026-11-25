package module_test

import (
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestLockViewRoundTripsURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mod.lock.kg")
	state := sampleLockState(t, module.LockKindModule)
	// One selected entry records the source URL; the aliased one omits the
	// redundant field, which is valid and must stay empty in the view.
	state.Locks[1].URL = ""
	if err := module.WriteLockStateFile(path, state); err != nil {
		t.Fatal(err)
	}
	got, err := module.ReadLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Entries["example.com/lib/a"].URL != "https://example.com/lib/a" {
		t.Fatalf("named URL = %q", got.Entries["example.com/lib/a"].URL)
	}
	if got.Entries["libalias"].URL != "" {
		t.Fatalf("aliased URL = %q", got.Entries["libalias"].URL)
	}
}
