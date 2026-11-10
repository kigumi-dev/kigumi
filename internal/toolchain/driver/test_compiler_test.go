package driver_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
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

var cCompilerOK bool

// init probes once with the runner's own PATH, before any test swaps in a zig
// shim: a hosted image can ship clang without a linker, and that result must
// not depend on which test happens to call testCompiler first.
func init() {
	opts := testCompilerOptions()
	if opts.CCompiler() != nil {
		cCompilerOK = linksTrivial(opts)
	}
}

func testCompilerOptions() buildconfig.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	return buildconfig.CompilerOptions{Host: buildconfig.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: filepath.SplitList(os.Getenv("PATH")), Home: home}
}

// testCompiler uses a compiler only when the runner's compiler can build a
// minimal module. Tests that need C then skip on a hermetic image without a
// linker instead of failing inside the driver.
func testCompiler() buildconfig.CompilerOptions {
	opts := testCompilerOptions()
	if opts.CCompiler() != nil && !cCompilerOK {
		opts.PathDirs = nil
	}
	return opts
}

func linksTrivial(opts buildconfig.CompilerOptions) bool {
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
