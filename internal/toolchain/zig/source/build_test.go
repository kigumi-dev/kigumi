package source_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/toolchain/zig/source"
	"kigumi/internal/toolchain/zig/store"
)

// fakeCmake stands in for a real build, writing a zig script reporting
// reportVersion to the install prefix — these tests exercise Build's orchestration, not a real LLVM build.
func fakeCmake(t *testing.T, reportVersion string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake cmake script needs a POSIX shell")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
prefix=""
for a in "$@"; do
  case "$a" in
    -DCMAKE_INSTALL_PREFIX=*) prefix="${a#-DCMAKE_INSTALL_PREFIX=}" ;;
  esac
done
if [ -n "$prefix" ]; then
  mkdir -p "$prefix"
  printf '#!/bin/sh\necho ` + reportVersion + `\n' > "$prefix/zig"
  chmod +x "$prefix/zig"
fi
exit 0
`
	path := filepath.Join(dir, "cmake")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func localCheckout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "--quiet")
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("# fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "CMakeLists.txt")
	run("commit", "--quiet", "-m", "fake zig checkout")
	return dir
}

func TestBuildInstallsAndRecordsCommit(t *testing.T) {
	fakeCmake(t, "0.17.0")
	checkout := localCheckout(t)
	root := t.TempDir()

	var notices []string
	m, err := source.Build(source.Request{Lookup: exec.LookPath, Checkout: checkout, Want: "0.17.0"}, root, func(s string) { notices = append(notices, s) })
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if m.Obtained != "source" || m.Commit == "" || m.Dirty {
		t.Fatalf("manifest = %+v", m)
	}
	if len(notices) == 0 {
		t.Fatal("expected at least one progress notice")
	}
	path, ok := store.Installed(root, "0.17.0")
	if !ok {
		t.Fatal("not installed after Build")
	}
	out, err := exec.Command(path, "version").Output()
	if err != nil || string(out) != "0.17.0\n" {
		t.Fatalf("installed zig: out=%q err=%v", out, err)
	}
}

func TestBuildRecordsDirtyCheckout(t *testing.T) {
	fakeCmake(t, "0.17.0")
	checkout := localCheckout(t)
	if err := os.WriteFile(filepath.Join(checkout, "uncommitted.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m, err := source.Build(source.Request{Lookup: exec.LookPath, Checkout: checkout, Want: "0.17.0"}, root, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !m.Dirty {
		t.Fatal("expected Dirty to be recorded")
	}
}

func TestBuildRejectsWrongVersion(t *testing.T) {
	fakeCmake(t, "0.17.0")
	checkout := localCheckout(t)
	root := t.TempDir()
	_, err := source.Build(source.Request{Lookup: exec.LookPath, Checkout: checkout, Want: "0.18.0"}, root, nil)
	if !errors.Is(err, source.ErrVersionMismatch) {
		t.Fatalf("expected ErrVersionMismatch, got %v", err)
	}
	var mismatch *source.VersionMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected a *VersionMismatch, got %T: %v", err, err)
	}
	if mismatch.Got != "0.17.0" || mismatch.Want != "0.18.0" {
		t.Fatalf("mismatch = %+v, want Got=0.17.0 Want=0.18.0", mismatch)
	}
	if _, ok := store.Installed(root, "0.18.0"); ok {
		t.Fatal("a rejected build must not leave a partial install")
	}
}

func TestBuildWithoutLookupFails(t *testing.T) {
	root := t.TempDir()
	_, err := source.Build(source.Request{Checkout: t.TempDir(), Want: "0.17.0"}, root, nil)
	if err == nil || !strings.Contains(err.Error(), "no tool search path given") {
		t.Fatalf("Build with a nil Lookup = %v, want the no tool search path error", err)
	}
	if _, ok := store.Installed(root, "0.17.0"); ok {
		t.Fatal("a failed build must not leave an install")
	}
}
