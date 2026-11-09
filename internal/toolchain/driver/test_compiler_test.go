package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/native/link"
)

func testCompilerAt(cwd string) buildconfig.CompilerOptions {
	o := testCompiler()
	o.Cwd = cwd
	return o
}

// requireZig skips a test that needs what only zig provides: compiler-rt for a
// freestanding archive, COFF for UEFI, the wasm32-wasi libc, a static musl, or
// lld. A host build reaching the same code paths still runs through clang.
func requireZig(t *testing.T) {
	t.Helper()
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	if !link.IsZig(cc) {
		t.Skipf("needs zig cc; the C compiler here is %s", filepath.Base(cc[0]))
	}
}

var probeOnce sync.Once
var probeOK bool

// testCompiler uses a compiler only when it can link a trivial program. A
// hosted image can ship clang without a linker; tests then take the same path
// as one without a C toolchain instead of failing inside the driver.
func testCompiler() buildconfig.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	dirs := filepath.SplitList(os.Getenv("PATH"))
	opts := buildconfig.CompilerOptions{Host: buildconfig.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: dirs, Home: home}
	if cc := opts.CCompiler(); cc != nil {
		probeOnce.Do(func() { probeOK = linksTrivial(cc, cwd) })
		if !probeOK {
			opts.PathDirs = nil
		}
	}
	return opts
}

func linksTrivial(cc []string, cwd string) bool {
	dir, err := os.MkdirTemp("", "kigumi-cc-probe")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "probe.c")
	out := filepath.Join(dir, "probe")
	if os.WriteFile(src, []byte("int main(void) { return 0; }\n"), 0o644) != nil {
		return false
	}
	cmd := exec.Command(cc[0], append(append([]string{}, cc[1:]...), src, "-o", out)...)
	cmd.Dir = cwd
	return cmd.Run() == nil
}
