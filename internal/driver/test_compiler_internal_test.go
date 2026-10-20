package driver

import (
	"os"
	"path/filepath"
)

func internalTestCompiler() CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	return CompilerOptions{Host: HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: filepath.SplitList(os.Getenv("PATH")), Home: home}
}
