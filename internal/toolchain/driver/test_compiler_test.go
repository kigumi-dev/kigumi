package driver_test

import (
	"os"
	"path/filepath"
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

func testCompiler() buildconfig.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	return buildconfig.CompilerOptions{Host: buildconfig.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: filepath.SplitList(os.Getenv("PATH")), Home: home}
}
