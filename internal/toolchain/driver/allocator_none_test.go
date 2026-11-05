package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// TestAllocatorNoneProfile carries `--allocator none` into the checker.
func TestAllocatorNoneProfile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"bare\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let xs = Array.of(1)\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	target := buildconfig.HostTarget()
	target.Allocator = "none"
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasErrors() || !strings.Contains(res.Render(), "needs the default allocator") {
		t.Fatalf("expected the allocator diagnostic:\n%s", res.Render())
	}
}

// A test block is a policy root like main and export(C): an allocating test
// must fail `kigumi test --allocator none`, not only `run`/`build`.
func TestAllocatorNoneCoversTestBlocks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"baretest\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("test \"allocates\" {\n    let xs = Array.of(1)\n    _ = xs\n}\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	target := buildconfig.HostTarget()
	target.Allocator = "none"
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasErrors() || !strings.Contains(res.Render(), "needs the default allocator") {
		t.Fatalf("a test block's allocation escaped --allocator none:\n%s", res.Render())
	}
}
