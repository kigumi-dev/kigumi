package module_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

// Missing files are the only errors read as "absent": an unreadable
// manifest is reported, and lock writes leave no temporary behind.
func TestModuleIOErrors(t *testing.T) {
	root := t.TempDir()
	if _, ok, err := module.ReadModFile(root); ok || err != nil {
		t.Fatalf("missing manifest: ok %v err %v", ok, err)
	}
	if os.Getuid() != 0 {
		os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"a\"\n    kigumi: \"0.1\"\n}\n"), 0o000)
		if _, _, err := module.ReadModFile(root); err == nil {
			t.Fatal("an unreadable manifest must be an error, not a missing one")
		}
		os.Remove(filepath.Join(root, "mod.kg"))
	}
	if err := module.WriteLock(root, module.Lock{Entries: map[string]module.LockEntry{"x": {Version: "v1.0.0", Commit: "c", Hash: "h"}}}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
	if _, err := module.ReadLock(root); err != nil {
		t.Fatal(err)
	}
}

// A chmod +x'd script (a `#!/usr/bin/env kigumi`
// direct-execution entry point) must keep its executable bit when `kigumi
// get` rewrites it to add a Require record.
func TestAppendScriptRequirePreservesExecutableBit(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	script := filepath.Join(t.TempDir(), "tool.kg")
	os.WriteFile(script, []byte("#!/usr/bin/env kigumi\nprint \"hi\"\n"), 0o644)
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := module.AppendScriptRequire(script, module.Require{Name: "github.com/acme/greet", Version: "v0.1.0"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(script)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("executable bit lost: mode is %v", fi.Mode().Perm())
	}
}
