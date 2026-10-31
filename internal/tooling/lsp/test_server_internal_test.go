package lsp

import (
	"os"
	"path/filepath"

	buildconfig "kigumi/internal/toolchain/config"
)

func internalTestCompiler() buildconfig.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	return buildconfig.CompilerOptions{Host: buildconfig.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: filepath.SplitList(os.Getenv("PATH")), Home: home}
}

func newServer(std string) *Server { return New(std).WithPackageCache("", internalTestCompiler()) }
