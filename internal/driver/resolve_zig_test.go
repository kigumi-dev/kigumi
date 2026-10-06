package driver_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func fakeZigReporting(t *testing.T, version string) string {
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

func fakeZigFailing(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake zig script needs a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "zig")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadPinned(t *testing.T, zigPin, zigPath string) (*driver.Module, error) {
	t.Helper()
	root := t.TempDir()
	src := "Module {\n    name: \"pinned\"\n    kigumi: \"0.1\"\n"
	if zigPin != "" {
		src += "    zig: \"" + zigPin + "\"\n"
	}
	src += "}\n"
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hello\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	return driver.LoadModule(root, driver.LoadOptions{StdRoot: std, ZigPath: zigPath})
}

func TestResolveZigMatch(t *testing.T) {
	zig := fakeZigReporting(t, "0.17.0")
	m, err := loadPinned(t, "0.17.0", zig)
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.ResolveZig()
	if err != nil || path != zig {
		t.Fatalf("path=%q err=%v, want %q nil", path, err, zig)
	}
}

func TestResolveZigMismatch(t *testing.T) {
	zig := fakeZigReporting(t, "0.16.0-dev.2287+eb3f16db5")
	m, err := loadPinned(t, "0.17.0", zig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.ResolveZig()
	if err == nil {
		t.Fatal("expected a mismatch error")
	}
	if !strings.Contains(err.Error(), "E1562") || !strings.Contains(err.Error(), "0.16.0-dev.2287+eb3f16db5") || !strings.Contains(err.Error(), "0.17.0") {
		t.Errorf("want E1562 naming both versions, got: %v", err)
	}
}

func TestResolveZigVersionUnknown(t *testing.T) {
	zig := fakeZigFailing(t)
	m, err := loadPinned(t, "0.17.0", zig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.ResolveZig()
	if err == nil {
		t.Fatal("expected a version-unknown error")
	}
	got := err.Error()
	if !strings.Contains(got, "E1561") {
		t.Fatalf("want E1561, got: %v", got)
	}
	if !strings.Contains(got, "exit status 1") {
		t.Errorf("want the real `zig version` failure in the message, got: %v", got)
	}
	if !strings.Contains(got, "0.17.0") {
		t.Errorf("want the pinned version named in the help, got: %v", got)
	}
	if strings.Contains(got, "{want}") || strings.Contains(got, "{detail}") {
		t.Errorf("want every template hole filled, got an unfilled placeholder: %v", got)
	}
}

func TestResolveZigMissingWithPin(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	m, err := loadPinned(t, "0.17.0", missing)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.ResolveZig()
	if err == nil || !strings.Contains(err.Error(), "E1560") {
		t.Errorf("want E1560 (zig missing), got: %v", err)
	}
}

func TestResolveZigOverrideBeatsPath(t *testing.T) {
	zig := fakeZigReporting(t, "0.17.0")
	t.Setenv("PATH", t.TempDir())
	m, err := loadPinned(t, "0.17.0", zig)
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.ResolveZig()
	if err != nil || path != zig {
		t.Fatalf("an explicit KIGUMI_ZIG must win over PATH: path=%q err=%v", path, err)
	}
}

func TestResolveZigUnpinnedNeedsNoZig(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m, err := loadPinned(t, "", "")
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.ResolveZig()
	if path != "" || err != nil {
		t.Fatalf("an unpinned module with no zig at all falls back to clang silently: path=%q err=%v", path, err)
	}
}

func TestCCompilerForRefusesWrongZig(t *testing.T) {
	zig := fakeZigReporting(t, "0.16.0-dev.2287+eb3f16db5")
	m, err := loadPinned(t, "0.17.0", zig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.CCompilerFor(m); err == nil {
		t.Fatal("expected CCompilerFor to refuse a zig that does not satisfy the pin")
	}
}
