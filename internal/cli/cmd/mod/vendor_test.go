package mod_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/toolchain/module"
)

// vendorFiles snapshots every regular file under dir, so two vendor runs can
// be compared byte for byte.
func vendorFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func vendorKeys(files map[string]string) []string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// `mod vendor` copies the locked dependency, records its content hash, and
// produces the same bytes on a second run. A build then works without the
// cache, and a tampered vendored file is refused.
func TestModVendorIsReproducibleAndUsable(t *testing.T) {
	root := t.TempDir()
	repo := clitest.Monorepo(t, root)
	app := filepath.Join(root, "app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", "Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \"v0.1.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import enclave from github.com/kigumilang/exp/sgx/enclave\nprint enclave.hello()\n"), 0o644)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	if code, out, errOut := clitest.Execute(t, "get", app); code != 0 {
		t.Fatalf("get: %s%s", out, errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "vendor", "-C", app); code != 0 {
		t.Fatalf("vendor: %s", errOut)
	}
	vendor := filepath.Join(app, "vendor")
	manifest := readFile(t, filepath.Join(vendor, module.VendorFileName))
	if !strings.Contains(manifest, "github.com/kigumilang/exp/sgx v0.1.0 sha1:") || !strings.Contains(manifest, " h1:") {
		t.Fatalf("vendor manifest should record the version, commit and content hash:\n%s", manifest)
	}
	if _, err := os.Stat(filepath.Join(vendor, "github.com", "kigumilang", "exp", "sgx", "enclave", "enclave.kg")); err != nil {
		t.Fatalf("vendored module content missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vendor, "github.com", "kigumilang", "exp", "sgx", module.OriginFileName)); err == nil {
		t.Fatal("the cache origin marker does not belong in vendor/")
	}
	first := vendorFiles(t, vendor)

	if code, _, errOut := clitest.Execute(t, "mod", "vendor", "-C", app); code != 0 {
		t.Fatalf("second vendor: %s", errOut)
	}
	second := vendorFiles(t, vendor)
	if strings.Join(vendorKeys(first), "\n") != strings.Join(vendorKeys(second), "\n") {
		t.Fatalf("vendor file set changed between runs:\n%v\n%v", vendorKeys(first), vendorKeys(second))
	}
	for _, key := range vendorKeys(first) {
		if first[key] != second[key] {
			t.Fatalf("%s changed between identical vendor runs", key)
		}
	}

	// With the cache gone, the checkout still builds from vendor/ and verifies.
	empty := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", empty)
	std, _ := filepath.Abs("../../../../std")
	if code, out, errOut := clitest.Execute(t, "--std", std, "run", app); code != 0 || out != "hi\n" {
		t.Fatalf("run from vendor: exit %d out %q err %q", code, out, errOut)
	}
	if code, out, _ := clitest.Execute(t, "mod", "verify", "-C", app); code != 0 || !strings.Contains(out, "ok (vendored)") {
		t.Fatalf("verify from vendor: exit %d %q", code, out)
	}

	// A vendored file edited in place is refused instead of compiled.
	tampered := filepath.Join(vendor, "github.com", "kigumilang", "exp", "sgx", "enclave", "enclave.kg")
	os.WriteFile(tampered, []byte("pub fn hello() -> String {\n    \"tampered\"\n}\n"), 0o644)
	if code, out, _ := clitest.Execute(t, "mod", "verify", "-C", app); code != 1 || !strings.Contains(out, "MISMATCH") {
		t.Fatalf("verify after tampering: exit %d %q", code, out)
	}
	if code, _, errOut := clitest.Execute(t, "--std", std, "run", app); code == 0 || !strings.Contains(errOut, module.VendorFileName) {
		t.Fatalf("run after tampering must fail naming the vendor manifest, exit %d: %q", code, errOut)
	}
}

// A hand-maintained vendor directory (resolved through mod.local.kg) is not
// silently replaced by `mod vendor`.
func TestModVendorKeepsManualTree(t *testing.T) {
	app := t.TempDir()
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	os.MkdirAll(filepath.Join(app, "vendor", "handmade"), 0o755)
	os.WriteFile(filepath.Join(app, "vendor", "handmade", "keep.txt"), []byte("keep\n"), 0o644)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if code, _, errOut := clitest.Execute(t, "mod", "vendor", "-C", app); code == 0 || !strings.Contains(errOut, module.VendorFileName) {
		t.Fatalf("a manual vendor tree must be refused, exit %d: %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(app, "vendor", "handmade", "keep.txt")); err != nil {
		t.Fatalf("the manual tree must be left alone: %v", err)
	}
}

// A locally replaced dependency is not vendored: the vendor tree must be the
// released module, not a developer's checkout.
func TestModVendorRefusesLocalReplace(t *testing.T) {
	root := t.TempDir()
	repo := clitest.Monorepo(t, root)
	app := filepath.Join(root, "app")
	checkout := filepath.Join(root, "checkout")
	os.MkdirAll(app, 0o755)
	os.MkdirAll(checkout, 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("app", "Require {\n    name: \"github.com/kigumilang/exp/sgx\"\n    version: \"v0.1.0\"\n    url: \"file://"+repo+"\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "mod.local.kg"), []byte("Replace { name: \"github.com/kigumilang/exp/sgx\", path: \"../checkout\" }\n"), 0o644)
	os.WriteFile(filepath.Join(checkout, "mod.kg"), clitest.Manifest("github.com/kigumilang/exp/sgx", ""), 0o644)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if code, _, errOut := clitest.Execute(t, "mod", "vendor", "-C", app); code == 0 || !strings.Contains(errOut, "replaced by mod.local.kg") {
		t.Fatalf("vendor must refuse a locally replaced module, exit %d: %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(app, "vendor")); err == nil {
		t.Fatal("a refused vendor run must not leave a vendor directory")
	}
}
