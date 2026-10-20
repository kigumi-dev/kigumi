package shared

import (
	"os"
	"path/filepath"
	"runtime"

	"kigumi/internal/driver"
)

// CompilerOptions reads the machine facts once so nothing below internal/cli asks the process.
func CompilerOptions() driver.CompilerOptions {
	cache, _ := os.UserCacheDir()
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return driver.CompilerOptions{
		Host:     driver.Target{OS: runtime.GOOS, Arch: runtime.GOARCH, Sys: "posix"},
		CacheDir: cache,
		Cwd:      cwd,
		PathDirs: filepath.SplitList(os.Getenv("PATH")),
		Home:     home,
	}
}
