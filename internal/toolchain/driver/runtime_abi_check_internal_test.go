package driver

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kigumi/internal/toolchain/artifact"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/native/link"
)

func TestLinkedRuntimeABICheckUsesCwd(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "stale.a"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := artifact.WriteFingerprintSidecar(filepath.Join(cwd, "stale.a"), artifact.Fingerprint{RuntimeAbi: "kgabi1:bogus"}); err != nil {
		t.Fatal(err)
	}
	m := &Module{compiler: buildconfig.CompilerOptions{Cwd: cwd}}
	cflags, err := link.ResolveFlagPaths(m.compiler, []string{"stale.a"})
	if err != nil {
		t.Fatal(err)
	}
	err = checkLinkedRuntimeABI(m, cflags, nil, "kgabi1:other")
	if err == nil || !strings.Contains(err.Error(), "E1880") {
		t.Fatalf("a stale archive named relative to Cwd must be refused, got %v", err)
	}
}

func TestLinkedArtifactsResolvesLinkSearchAgainstCwd(t *testing.T) {
	cwd := t.TempDir()
	m := &Module{compiler: buildconfig.CompilerOptions{Cwd: cwd}, Links: []module.Link{{Library: "foo", Search: "libs"}, {Library: "bar", Search: "/abs"}}}
	got := linkedArtifacts(m, nil, nil)
	want := []string{filepath.Join(cwd, "libs", "libfoo.a"), filepath.Join("/abs", "libbar.a")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linkedArtifacts = %q, want %q", got, want)
	}
	if err := os.MkdirAll(filepath.Join(cwd, "libs"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(cwd, "libs", "libfoo.a")
	if err := os.WriteFile(stale, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := artifact.WriteFingerprintSidecar(stale, artifact.Fingerprint{RuntimeAbi: "kgabi1:bogus"}); err != nil {
		t.Fatal(err)
	}
	if err := checkLinkedRuntimeABI(m, nil, nil, "kgabi1:other"); err == nil || !strings.Contains(err.Error(), "E1880") {
		t.Fatalf("a stale library found through a relative Search must be refused, got %v", err)
	}
}
