package zigtool_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"kigumi/internal/toolchain/zig"
)

func fakeZig(t *testing.T, version string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake zig script needs a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "zig")
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo '" + version + "'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckZigMatch(t *testing.T) {
	zig := fakeZig(t, "0.17.0")
	c := zigtool.CheckZig(nil, zig, "0.17.0")
	if c.Path != zig || c.Version != "0.17.0" || c.Mismatch || c.Err != nil {
		t.Fatalf("%+v", c)
	}
}

func TestCheckZigMismatch(t *testing.T) {
	zig := fakeZig(t, "0.16.0-dev.2287+eb3f16db5")
	c := zigtool.CheckZig(nil, zig, "0.17.0")
	if !c.Mismatch || c.Version != "0.16.0-dev.2287+eb3f16db5" || c.Want != "0.17.0" || c.Err != nil {
		t.Fatalf("%+v", c)
	}
}

func TestCheckZigUnpinnedSkipsVersion(t *testing.T) {
	zig := fakeZig(t, "0.17.0")
	c := zigtool.CheckZig(nil, zig, "")
	if c.Path != zig || c.Version != "" || c.Mismatch || c.Err != nil {
		t.Fatalf("an unpinned check must not even run `zig version`: %+v", c)
	}
}

func TestCheckZigMissingNoPin(t *testing.T) {
	c := zigtool.CheckZig(zigtool.InDirs([]string{t.TempDir()}), "", "")
	if c != (zigtool.Check{}) {
		t.Fatalf("no override, no pin, no zig on the search path: the zero Check, got %+v", c)
	}
}

func TestCheckZigFindsZigOnTheSearchPath(t *testing.T) {
	zig := fakeZig(t, "0.17.0")
	c := zigtool.CheckZig(zigtool.InDirs([]string{filepath.Dir(zig)}), "", "")
	if c.Path != zig || c.Err != nil {
		t.Fatalf("a zig on the search path must be found: %+v", c)
	}
}

func TestCheckZigMissingWithPin(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	c := zigtool.CheckZig(nil, missing, "0.17.0")
	if c.Path != "" || c.Err == nil || c.Want != "0.17.0" {
		t.Fatalf("%+v", c)
	}
}

func TestCheckZigExplicitOverrideWins(t *testing.T) {
	zig := fakeZig(t, "0.17.0")
	// PATH is left untouched; an explicit path must still be the one used.
	c := zigtool.CheckZig(nil, zig, "0.17.0")
	if c.Path != zig {
		t.Fatalf("explicit override was not used: %+v", c)
	}
}

func TestCheckZigExplicitInvalidErrorsEvenUnpinned(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	c := zigtool.CheckZig(nil, missing, "")
	if c.Err == nil {
		t.Fatal("an explicit KIGUMI_ZIG that does not resolve must error even with no pin")
	}
}

func TestVersionTrimsAndCaches(t *testing.T) {
	zig := fakeZig(t, "0.17.0")
	v1, err := zigtool.Version(zig)
	if err != nil || v1 != "0.17.0" {
		t.Fatalf("v=%q err=%v", v1, err)
	}
	if err := os.Chmod(zig, 0o644); err != nil {
		t.Fatal(err)
	}
	v2, err := zigtool.Version(zig)
	if err != nil || v2 != v1 {
		t.Fatalf("a cached version must not re-invoke a now non-executable binary: v=%q err=%v", v2, err)
	}
}

func TestFingerprintDiffersByMtime(t *testing.T) {
	zig := fakeZig(t, "0.17.0")
	fp1, err := zigtool.Fingerprint(zig)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(zig, later, later); err != nil {
		t.Fatal(err)
	}
	fp2, err := zigtool.Fingerprint(zig)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 == fp2 {
		t.Fatal("fingerprint must change when mtime changes")
	}
}

func TestInDirsSkipsRelativeEntries(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "zig"), []byte("#!/bin/sh\n"), 0o755)
	t.Chdir(filepath.Dir(d))
	if p, err := zigtool.InDirs([]string{filepath.Base(d)})("zig"); err == nil {
		t.Fatalf("relative entry found %s", p)
	}
	if _, err := zigtool.InDirs([]string{d})("zig"); err != nil {
		t.Fatal(err)
	}
}
