package driver

import (
	"os"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
)

// TestRuntimeFingerprintCoversTargetAndLayout: two targets or two layout-changing CFLAGS never share a runtime fingerprint, while flags
// that leave the layout alone keep it, so mixing archives across them is E1880 and not a silent layout disagreement.
func TestRuntimeFingerprintCoversTargetAndLayout(t *testing.T) {
	t.Parallel()
	host := buildconfig.HostTarget()
	flags := []string{"KIGUMI_UNWIND"}
	base := runtimeFingerprint(host, flags, nil)

	other := buildconfig.HostTarget()
	other.Arch = "arm64"
	if runtimeFingerprint(other, flags, nil) == base {
		t.Error("another target must not share the runtime fingerprint")
	}
	if abiBase(other) == abiBase(host) {
		t.Error("another target must not share the ABI base")
	}
	for _, cflags := range [][]string{
		{"-fshort-enums"},
		{"-fshort-wchar"},
		{"-fpack-struct=1"},
		{"-m32"},
		{"-mabi=ilp32"},
	} {
		if runtimeFingerprint(host, flags, cflags) == base {
			t.Errorf("%v must move the runtime fingerprint", cflags)
		}
	}
	for _, cflags := range [][]string{{"-O2"}, {"-Wall"}, {"-I/usr/include"}, {"-DFOO=1"}} {
		if got := runtimeFingerprint(host, flags, cflags); got != base {
			t.Errorf("%v must not move the runtime fingerprint, got %s want %s", cflags, got, base)
		}
	}
}

// TestLinkedArtifactsResolvesLibraryFlags: an archive reached through -l is ABI-checked wherever the search path holds it, in the forms
// the driver passes through.
func TestLinkedArtifactsResolvesLibraryFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lib := filepath.Join(dir, "libprobe.a")
	if err := os.WriteFile(lib, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "libdecoy.a"), []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Module{}
	for _, flags := range [][]string{
		{"-L" + dir, "-lprobe"},
		{"-L", dir, "-l", "probe"},
		{"-Wl,-L" + dir, "-Wl,-lprobe"},
		{"-Wl,-L," + dir + ",-lprobe"},
		{"-Wl,--library-path=" + dir, "-lprobe"},
		{filepath.Join(other, "libdecoy.a"), "-L" + dir, "-lprobe"},
	} {
		got := linkedArtifacts(m, flags, nil)
		found := false
		for _, p := range got {
			if p == lib {
				found = true
			}
		}
		if !found {
			t.Errorf("%v: linkedArtifacts = %v, want it to contain %s", flags, got, lib)
		}
	}
	// A library no -L directory holds (a system -lm, say) is not this build's to inspect.
	if got := linkedArtifacts(m, []string{"-lm"}, nil); len(got) != 0 {
		t.Errorf("an unresolved -l must add nothing, got %v", got)
	}
	// The first directory holding the archive wins, as it does for ld.
	near := t.TempDir()
	nearLib := filepath.Join(near, "libprobe.a")
	if err := os.WriteFile(nearLib, []byte("near"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := linkedArtifacts(m, []string{"-L" + near, "-L" + dir, "-lprobe"}, nil)
	if len(got) != 1 || got[0] != nearLib {
		t.Errorf("linkedArtifacts = %v, want just the first match %s", got, nearLib)
	}
}
