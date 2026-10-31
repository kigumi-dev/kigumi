package driver

import (
	"errors"
	"path/filepath"
	"runtime"
)

// CompilerOptions holds process facts that reach the compiler; the compiler reads the process through none of them when one is zero.
type CompilerOptions struct {
	// Host is the OS and architecture the default target and a Link search use.
	Host Target
	// CacheDir is the per-user cache root.
	CacheDir string
	// Cwd is the directory a running program's relative paths resolve against.
	Cwd string
	// PathDirs is the tool search path (zig, clang, ld); nil and empty both find nothing.
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
	if o.CacheDir == "" {
		return "", errors.New("CompilerOptions.CacheDir is required to locate the cache")
	}
	return o.CacheDir, nil
}

// cwdFor is the directory a run uses: the run's own choice, else the options'.
func (m *Module) cwdFor(own string) string {
	if own != "" {
		return own
	}
	return m.compiler.Cwd
}

// Abs resolves a relative p against the options' Cwd; an empty Cwd is an error.
func (o CompilerOptions) Abs(p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	if o.Cwd == "" {
		return "", errors.New("CompilerOptions.Cwd is required to resolve " + p)
	}
	return filepath.Join(o.Cwd, p), nil
}
