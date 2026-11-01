package zig_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/toolchain/zig/store"
)

// seedZigStore stands in for a real download; signature verification is
// fetch's job (internal/toolchain/zig/fetch), not tested here.
func seedZigStore(t *testing.T, root, version string) {
	t.Helper()
	st, err := store.Stage(root, version)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.BinaryPath(st.Dir), []byte("#!/bin/sh\necho "+version+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := store.Manifest{Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, InstalledAt: time.Now(), Obtained: "release"}
	if err := store.WriteManifest(st.Dir, m); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestZigInstallListRemove(t *testing.T) {
	home := t.TempDir()
	seedZigStore(t, home, "9.4.1")

	code, out, errOut := clitest.Execute(t, "zig", "install", "9.4.1", "--zig-home", home)
	if code != 0 || !strings.Contains(out, "9.4.1") {
		t.Fatalf("install: code=%d out=%q err=%q", code, out, errOut)
	}

	code, out, errOut = clitest.Execute(t, "zig", "list", "--zig-home", home)
	if code != 0 || !strings.Contains(out, "9.4.1") {
		t.Fatalf("list: code=%d out=%q err=%q", code, out, errOut)
	}

	code, out, errOut = clitest.Execute(t, "zig", "remove", "9.4.1", "--zig-home", home)
	if code != 0 {
		t.Fatalf("remove: %s%s", out, errOut)
	}
	code, out, errOut = clitest.Execute(t, "zig", "list", "--zig-home", home)
	if code != 0 || strings.Contains(out, "9.4.1") {
		t.Fatalf("expected 9.4.1 gone after remove: out=%q err=%q", out, errOut)
	}
}

func TestZigRemoveRejectsPathTraversal(t *testing.T) {
	home := t.TempDir()
	victim := t.TempDir()
	if err := os.WriteFile(filepath.Join(victim, "precious.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(home, victim)
	if err != nil {
		t.Fatal(err)
	}

	code, _, errOut := clitest.Execute(t, "zig", "remove", rel, "--zig-home", home)
	if code == 0 {
		t.Fatalf("expected an error for a traversal version string, got exit 0 (stderr=%q)", errOut)
	}
	if _, err := os.Stat(filepath.Join(victim, "precious.txt")); err != nil {
		t.Fatalf("victim file must survive: %v", err)
	}
}

func TestZigWhichReportsManagedStore(t *testing.T) {
	home := t.TempDir()
	seedZigStore(t, home, "9.4.2")

	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n    zig: \"9.4.2\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(mod)

	code, out, errOut := clitest.Execute(t, "zig", "which", "--zig-home", home)
	if code != 0 {
		t.Fatalf("which: %s%s", out, errOut)
	}
	if !strings.Contains(out, "managed store") || !strings.Contains(out, "satisfies pin 9.4.2") {
		t.Fatalf("which output = %q", out)
	}
}

func TestZigListNeedsMarksModulePin(t *testing.T) {
	home := t.TempDir()
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n    zig: \"9.4.3\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(mod)
	code, out, errOut := clitest.Execute(t, "zig", "list", "--zig-home", home)
	if code != 0 || !strings.Contains(out, "9.4.3") || !strings.Contains(out, "not installed") {
		t.Fatalf("list: code=%d out=%q err=%q", code, out, errOut)
	}
}

func fakeCmakeOnPATH(t *testing.T, reportVersion string) {
	t.Helper()
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

func localGitCheckout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	runGit("init", "--quiet")
	os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("# fake\n"), 0o644)
	runGit("add", "CMakeLists.txt")
	runGit("commit", "--quiet", "-m", "fake zig checkout")
	return dir
}

func TestZigInstallFromSource(t *testing.T) {
	fakeCmakeOnPATH(t, "0.18.0-dev.1+deadbeef")
	checkout := localGitCheckout(t)
	home := t.TempDir()

	code, out, errOut := clitest.Execute(t, "zig", "install", "0.18.0-dev.1+deadbeef", "--source", checkout, "--zig-home", home)
	if code != 0 {
		t.Fatalf("install --source: %s%s", out, errOut)
	}
	if !strings.Contains(out, "built from source") {
		t.Fatalf("install --source output = %q", out)
	}

	code, out, errOut = clitest.Execute(t, "zig", "list", "--zig-home", home)
	if code != 0 || !strings.Contains(out, "0.18.0-dev.1+deadbeef") || !strings.Contains(out, "source") {
		t.Fatalf("list after source install: code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestZigInstallFromSourceRejectsWrongVersion(t *testing.T) {
	fakeCmakeOnPATH(t, "0.18.0-dev.1+deadbeef")
	checkout := localGitCheckout(t)
	home := t.TempDir()

	code, errOut := clitest.ExecuteReal(t, "zig", "install", "0.18.0-dev.2+other", "--source", checkout, "--zig-home", home)
	if code == 0 || !strings.Contains(errOut, "E1565") {
		t.Fatalf("want E1565 (source build unverified), code=%d err=%q", code, errOut)
	}
	if strings.Contains(errOut, "built zig does not report the requested version: got") {
		t.Fatalf("want {got} filled with just the reported version, not the wrapped error text: %q", errOut)
	}
}
