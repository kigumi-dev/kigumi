package link_test

import (
	"os"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/native/link"
)

func TestIsHostTarget(t *testing.T) {
	if !link.IsHostTarget(testCompiler(), buildconfig.Target{}) {
		t.Error("the zero Target should count as host")
	}
	if !link.IsHostTarget(testCompiler(), buildconfig.HostTarget()) {
		t.Error("HostTarget() should count as host")
	}
	if link.IsHostTarget(testCompiler(), buildconfig.Target{Triple: "x86_64-freestanding"}) {
		t.Error("an explicit Triple should not count as host")
	}
}

func TestResolveLinkIdentitySearchPath(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "libfoo.so")
	if err := os.WriteFile(lib, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, ok := link.ResolveLinkIdentity(testCompiler(), link.Library{Name: "foo", Search: dir}, buildconfig.Target{})
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
	id2, ok := link.ResolveLinkIdentity(testCompiler(), link.Library{Name: "foo", Search: dir}, buildconfig.Target{})
	if !ok || id2.Digest == id.Digest {
		t.Errorf("changing the library's bytes should change its digest: before=%s after=%s", id.Digest, id2.Digest)
	}
}

func TestResolveLinkIdentityUnresolved(t *testing.T) {
	if _, ok := link.ResolveLinkIdentity(testCompiler(), link.Library{Name: "kigumi_test_definitely_missing_xyz"}, buildconfig.Target{}); ok {
		t.Error("a library absent from every search directory should not resolve")
	}
	if _, ok := link.ResolveLinkIdentity(testCompiler(), link.Library{Name: "m"}, buildconfig.Target{Triple: "x86_64-freestanding"}); ok {
		t.Error("a non-host Target should not resolve against this host's search list")
	}
}
