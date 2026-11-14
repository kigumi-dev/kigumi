package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
)

// CompilerOptions reads the machine facts once so nothing below internal/cli asks the process.
func CompilerOptions() buildconfig.CompilerOptions {
	cache, _ := os.UserCacheDir()
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return buildconfig.CompilerOptions{
		Host:     buildconfig.HostTarget(),
		CacheDir: cache,
		Cwd:      cwd,
		PathDirs: filepath.SplitList(os.Getenv("PATH")),
		Home:     home,
	}
}

// LookPath resolves a program against the CompilerOptions search path, so
// commands do not read the process PATH after startup.
func LookPath(o buildconfig.CompilerOptions, name string) (string, error) {
	if filepath.IsAbs(name) || strings.ContainsRune(name, filepath.Separator) {
		if isExecutable(name) {
			return name, nil
		}
		return "", fmt.Errorf("%s: not executable", name)
	}
	for _, dir := range o.PathDirs {
		p := filepath.Join(dir, name)
		if isExecutable(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s: executable file not found in $PATH", name)
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// StateHome is the base directory for user state files.
func StateHome(o buildconfig.CompilerOptions) string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return d
	}
	if o.Home != "" {
		return filepath.Join(o.Home, ".local", "state")
	}
	return ""
}
