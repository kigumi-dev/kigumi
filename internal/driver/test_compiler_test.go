package driver_test

import (
	"os"
	"path/filepath"

	"kigumi/internal/driver"
)

func testCompilerAt(cwd string) driver.CompilerOptions {
	o := testCompiler()
	o.Cwd = cwd
	return o
}

func testCompiler() driver.CompilerOptions {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	return driver.CompilerOptions{Host: driver.HostTarget(), CacheDir: cache, Cwd: cwd, PathDirs: filepath.SplitList(os.Getenv("PATH")), Home: home}
}
