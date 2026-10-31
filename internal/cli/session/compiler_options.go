package shared

import (
	"os"
	"path/filepath"
	"runtime"

	buildconfig "kigumi/internal/toolchain/config"
)

// CompilerOptions reads the machine facts once so nothing below internal/cli asks the process.
func CompilerOptions() buildconfig.CompilerOptions {
	cache, _ := os.UserCacheDir()
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return buildconfig.CompilerOptions{
		Host:     buildconfig.Target{OS: runtime.GOOS, Arch: runtime.GOARCH, Sys: "posix"},
		CacheDir: cache,
		Cwd:      cwd,
		PathDirs: filepath.SplitList(os.Getenv("PATH")),
		Home:     home,
	}
}
