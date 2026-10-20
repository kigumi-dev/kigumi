package lsp

import (
	"os"
	"path/filepath"

	"kigumi/internal/driver"
)

func internalTestCompiler() driver.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	return driver.CompilerOptions{Host: driver.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: filepath.SplitList(os.Getenv("PATH")), Home: home}
}

func newServer(std string) *Server { return New(std).WithPackageCache("", internalTestCompiler()) }
