package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// TestValidModuleNameRejectsNonCanonicalHost checks that a non-lowercase host segment is rejected with a suggested canonical spelling.
func TestValidModuleNameRejectsNonCanonicalHost(t *testing.T) {
	t.Parallel()
	err := driver.ValidModuleName("GitHub.com/acme/x")
	if err == nil {
		t.Fatal("a non-lowercase host must be rejected")
	}
	if !strings.Contains(err.Error(), "github.com/acme/x") {
		t.Fatalf("error must suggest the canonical spelling, got: %v", err)
	}
	if err := driver.ValidModuleName("github.com/acme/x"); err != nil {
		t.Fatalf("the canonical spelling must be accepted: %v", err)
	}
}

// TestValidModuleNameRejectsTrailingDot covers the other modules_v2 rule:
// no segment may end with `.`.
func TestValidModuleNameRejectsTrailingDot(t *testing.T) {
	t.Parallel()
	if err := driver.ValidModuleName("example.com./acme"); err == nil {
		t.Fatal("a trailing dot on a segment must be rejected")
	}
}

// TestValidModuleNameLocalNameCaseSensitive checks that CanonicalModuleName leaves an unqualified name's case untouched; only the host is case-insensitive.
func TestValidModuleNameLocalNameCaseSensitive(t *testing.T) {
	t.Parallel()
	if err := driver.ValidModuleName("MyApp/sub"); err != nil {
		t.Fatalf("a local module name's case must not be forced canonical: %v", err)
	}
}

// TestCacheDirReservedComponent checks that a Windows-reserved device name (legal under ValidModuleName) doesn't come out as a bare reserved name.
func TestCacheDirReservedComponent(t *testing.T) {
	t.Parallel()
	dir, err := driver.CacheDir(driver.Require{Name: "example.com/con", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dir, "/con@") || strings.HasSuffix(dir, "con") {
		t.Fatalf("a bare reserved component must not appear in the cache path: %s", dir)
	}
	if !strings.Contains(dir, "!!con") {
		t.Fatalf("the reserved component must be escaped, got: %s", dir)
	}
}

// TestCacheDirCaseInjective checks that two module names differing only by case never collide with some third valid name's cache directory.
func TestCacheDirCaseInjective(t *testing.T) {
	t.Parallel()
	names := []string{"example.com/Acme/Lib", "example.com/acme/lib", "example.com/ACME/LIB", "example.com/con", "example.com/Con", "example.com/CON.ini"}
	seen := map[string]string{}
	for _, n := range names {
		dir, err := driver.CacheDir(driver.Require{Name: n, Version: "v1.0.0"})
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if prev, ok := seen[dir]; ok {
			t.Fatalf("%s and %s collide at cache path %s", n, prev, dir)
		}
		seen[dir] = n
	}
}
