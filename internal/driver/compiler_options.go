package driver

import (
	"os"
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
