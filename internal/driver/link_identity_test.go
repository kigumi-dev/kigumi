package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func TestIsHostTarget(t *testing.T) {
	if !driver.IsHostTarget(testCompiler(), driver.Target{}) {
		t.Error("the zero Target should count as host")
	}
	if !driver.IsHostTarget(testCompiler(), driver.HostTarget()) {
		t.Error("HostTarget() should count as host")
	}
	if driver.IsHostTarget(testCompiler(), driver.Target{Triple: "x86_64-freestanding"}) {
		t.Error("an explicit Triple should not count as host")
	}
}

func TestResolveLinkIdentitySearchPath(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "libfoo.so")
	if err := os.WriteFile(lib, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, ok := driver.ResolveLinkIdentity(testCompiler(), driver.Link{Library: "foo", Search: dir}, driver.Target{})
	if !ok {
		t.Fatalf("expected foo to resolve from %s", dir)
	}
	if id.ResolvedPath != lib {
		t.Errorf("ResolvedPath = %q, want %q", id.ResolvedPath, lib)
	}
	if id.Digest == "" {
		t.Error("expected a non-empty digest")
	}

	os.WriteFile(lib, []byte("two"), 0o644)
	id2, ok := driver.ResolveLinkIdentity(testCompiler(), driver.Link{Library: "foo", Search: dir}, driver.Target{})
	if !ok || id2.Digest == id.Digest {
		t.Errorf("changing the library's bytes should change its digest: before=%s after=%s", id.Digest, id2.Digest)
	}
}

func TestResolveLinkIdentityUnresolved(t *testing.T) {
	if _, ok := driver.ResolveLinkIdentity(testCompiler(), driver.Link{Library: "kigumi_test_definitely_missing_xyz"}, driver.Target{}); ok {
		t.Error("a library absent from every search directory should not resolve")
	}
	if _, ok := driver.ResolveLinkIdentity(testCompiler(), driver.Link{Library: "m"}, driver.Target{Triple: "x86_64-freestanding"}); ok {
		t.Error("a non-host Target should not resolve against this host's search list")
	}
}

func TestLinkIdentityViolationsReleaseOnly(t *testing.T) {
	m := &driver.Module{Name: "app", Links: []driver.Link{{Library: "kigumi_test_definitely_missing_xyz"}}}
	vs := driver.LinkIdentityViolations(m, driver.Target{})
	if len(vs) != 1 || vs[0].ID != "release-native-link-identity-unknown" {
		t.Fatalf("want one release-native-link-identity-unknown violation, got %+v", vs)
	}
	dev := driver.EvaluateRelease(m, driver.Development(), vs...)
	if len(dev.Warnings) != 1 || len(dev.Errors) != 0 {
		t.Errorf("development should warn, not fail: %+v", dev)
	}
	rel := driver.EvaluateRelease(m, driver.Release(), vs...)
	if len(rel.Errors) != 1 {
		t.Errorf("release should refuse an unidentified Link: %+v", rel)
	}

	resolvable := &driver.Module{Name: "app", Links: []driver.Link{{Library: "m"}}}
	if vs := driver.LinkIdentityViolations(resolvable, driver.Target{}); len(vs) != 0 {
		t.Errorf("libm should resolve on this host, got %+v", vs)
	}
}
