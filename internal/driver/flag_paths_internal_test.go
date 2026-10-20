package driver

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveFlagPaths(t *testing.T) {
	cwd := t.TempDir()
	o := CompilerOptions{Cwd: cwd}
	abs := func(p string) string { return filepath.Join(cwd, p) }
	in := []string{"-Iinc", "-I", "inc2", "-Llib", "-L", "lib2", "-isystem", "sys", "-Tscript.ld", "-T", "s2.ld", "extra.c", "obj.o", "libx.a", "link.ld", "-lm", "-DX=1", "-Wl,--gc-sections", "-I/abs", "/abs/y.c"}
	want := []string{"-I" + abs("inc"), "-I", abs("inc2"), "-L" + abs("lib"), "-L", abs("lib2"), "-isystem", abs("sys"), "-T" + abs("script.ld"), "-T", abs("s2.ld"), abs("extra.c"), abs("obj.o"), abs("libx.a"), abs("link.ld"), "-lm", "-DX=1", "-Wl,--gc-sections", "-I/abs", "/abs/y.c"}
	got, err := resolveFlagPaths(o, in)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveFlagPaths = %q, %v\nwant %q", got, err, want)
	}
	if _, err := resolveFlagPaths(CompilerOptions{}, []string{"-Iinc"}); err == nil {
		t.Error("a relative path with an empty Cwd must fail")
	}
	if got, err := resolveFlagPaths(CompilerOptions{}, []string{"-O2", "-I/abs"}); err != nil || len(got) != 2 {
		t.Errorf("flags without relative paths need no Cwd: %q %v", got, err)
	}
}

func TestLinkedRuntimeABICheckUsesCwd(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "stale.a"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFingerprintSidecar(filepath.Join(cwd, "stale.a"), ArtifactFingerprint{RuntimeAbi: "kgabi1:bogus"}); err != nil {
		t.Fatal(err)
	}
	m := &Module{compiler: CompilerOptions{Cwd: cwd}}
	cflags, err := resolveFlagPaths(m.compiler, []string{"stale.a"})
	if err != nil {
		t.Fatal(err)
	}
	err = checkLinkedRuntimeABI(m, cflags, nil, "kgabi1:other")
	if err == nil || !strings.Contains(err.Error(), "E1880") {
		t.Fatalf("a stale archive named relative to Cwd must be refused, got %v", err)
	}
}
