package driver

import (
	"os"
	"path/filepath"
	"runtime"
)

// CompilerOptions holds process facts that reach the compiler; a zero field falls back to the process only here.
type CompilerOptions struct {
	// Host is the OS and architecture the default target and a Link search use.
	Host Target
	// CacheDir is the per-user cache root.
	CacheDir string
	// Cwd is the directory a running program's relative paths resolve against.
	Cwd string
	// PathDirs is the tool search path (zig, clang, ld); nil falls back to the process PATH, empty finds nothing.
	PathDirs []string
	// Home is the user's home directory, the base of the managed zig store.
	Home string
}

// HostTarget is the machine kigumi runs on, the fallback for CompilerOptions.Host.
func HostTarget() Target {
	return Target{OS: runtime.GOOS, Arch: runtime.GOARCH, Sys: "posix"}
}

func (o CompilerOptions) HostOrAmbient() Target {
	if o.Host.OS != "" {
		return o.Host
	}
	return HostTarget()
}

func (o CompilerOptions) cacheRoot() (string, error) {
	if o.CacheDir != "" {
		return o.CacheDir, nil
	}
	return os.UserCacheDir()
}

// cwdFor is the directory a run uses: the run's own choice, else the options'.
func (m *Module) cwdFor(own string) string {
	if own != "" {
		return own
	}
	if m.compiler.Cwd != "" {
		return m.compiler.Cwd
	}
	cwd, _ := os.Getwd()
	return cwd
}

// Abs resolves p against the options' Cwd, or the process directory only when none was given.
func (o CompilerOptions) Abs(p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	cwd := o.Cwd
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return "", err
		}
	}
	return filepath.Join(cwd, p), nil
}
