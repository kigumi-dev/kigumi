package driver

import (
	"os"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
)

// TestModuleLoadOptionsKeepThePolicyFlags guards the settings a reload for an
// artifact or a plan must carry: --no-local, --offline and the zig/package
// cache locations cannot silently fall back to their defaults.
func TestModuleLoadOptionsKeepThePolicyFlags(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	zigHome, pkgCache := t.TempDir(), t.TempDir()
	m, err := LoadModule(root, LoadOptions{
		NoLocal:      true,
		Offline:      true,
		ZigHome:      zigHome,
		ZigIndexURL:  "https://index.invalid/",
		PackageCache: pkgCache,
		Compiler:     buildconfig.CompilerOptions{Cwd: root, Home: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := m.loadOptions()
	if !got.NoLocal || !got.Offline || got.ZigHome != zigHome || got.PackageCache != pkgCache || got.ZigIndexURL != "https://index.invalid/" {
		t.Fatalf("reload options dropped a flag: %+v", got)
	}
	if got.RootDirs != nil || got.ExtraSources != nil || got.Entry != "" {
		t.Fatalf("an artifact reload must scope its own roots: %+v", got)
	}
}
