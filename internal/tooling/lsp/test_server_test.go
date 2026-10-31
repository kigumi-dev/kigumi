package lsp_test

import (
	"os"
	"path/filepath"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/tooling/lsp"
)

func testCompiler() driver.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	return driver.CompilerOptions{Host: driver.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: filepath.SplitList(os.Getenv("PATH")), Home: home}
}

func newServer(std string) *lsp.Server { return lsp.New(std).WithPackageCache("", testCompiler()) }
