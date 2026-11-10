package driver_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
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

var (
	probeMu    sync.Mutex
	probeCache = map[string]bool{}
)

// testCompiler uses a compiler only when it can link a trivial program. A
// hosted image can ship clang without a linker; tests then take the same path
// as one without a C toolchain instead of failing inside the driver. The
// cache is keyed by the search path: tests that prepend a zig shim probe that
// compiler and must not lend its result to the plain clang on PATH afterwards.
func testCompiler() buildconfig.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	dirs := filepath.SplitList(os.Getenv("PATH"))
	opts := buildconfig.CompilerOptions{Host: buildconfig.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: dirs, Home: home}
	if cc := opts.CCompiler(); cc != nil {
		key := strings.Join(dirs, string(os.PathListSeparator)) + "\x00" + cc[0]
		probeMu.Lock()
		ok, seen := probeCache[key]
		if !seen {
			ok = linksTrivial(opts, cwd)
			probeCache[key] = ok
		}
		probeMu.Unlock()
		if !ok {
			opts.PathDirs = nil
		}
	}
	return opts
}

// linksTrivial builds a real minimal module: a compiler on PATH may link a
// bare C file while the driver's host target or a reproducible build still
// fails, so the probe exercises the same entry point the tests do.
func linksTrivial(opts buildconfig.CompilerOptions, cwd string) bool {
	dir, err := os.MkdirTemp("", "kigumi-cc-probe")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	if os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"probe\"\n    kigumi: \"0.1\"\n}\n"), 0o644) != nil {
		return false
	}
	if os.WriteFile(filepath.Join(dir, "main.kg"), []byte("let x = 1\n"), 0o644) != nil {
		return false
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return false
	}
	std := filepath.Join(filepath.Dir(file), "..", "..", "..", "std")
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: opts, StdRoot: std, Entry: "main.kg"})
	if err != nil {
		return false
	}
	built, err := driver.BuildWith(m, filepath.Join(dir, "probe"), io.Discard, driver.BuildOptions{})
	return err == nil && built
}
