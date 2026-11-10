package driver

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"kigumi/internal/toolchain/buildgraph"
	buildconfig "kigumi/internal/toolchain/config"
)

// zigCacheDir is where the built-in toolchain Framework's zig-backed tools
// cache their work: under the build's own output directory, the last of
// Graph.Roots, rather than a host directory the
// tool env allowlist otherwise hides from them (there is no HOME).
func zigCacheDir(g *buildgraph.Graph) string {
	if len(g.Roots) == 0 {
		return ""
	}
	return filepath.Join(g.Roots[len(g.Roots)-1], ".kigumi-zig-cache")
}

// resolveProgram turns a tool's declared program into the executable path the
// launch will use: a bare name is searched in the compiler options' path
// directories, never the driver's own environment, and a path that already
// names a file is used as given. ok is false when a bare name cannot be found.
func resolveProgram(o buildconfig.CompilerOptions, program string) (string, bool) {
	if filepath.IsAbs(program) || strings.ContainsAny(program, `\/`) {
		return program, true
	}
	p, err := o.LookTool(program)
	if err != nil {
		return program, false
	}
	return p, true
}

func placeholderIndex(a string, n int) (int, error) {
	i, err := strconv.Atoi(a[strings.Index(a, ":")+1:])
	if err != nil || i < 0 || i >= n {
		return 0, fmt.Errorf("placeholder %s is malformed or out of range (%d declared)", a, n)
	}
	return i, nil
}

// withinSandbox mirrors the VM's check for the executing side: symlinks
// are followed on both the path and the roots.
func withinSandbox(roots []string, path string) bool {
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	path = resolve(path)
	for _, r := range roots {
		rel, err := filepath.Rel(resolve(r), path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// toolEnv is the allowlisted environment plus the target as explicit
// input; an empty allowlist still yields a defined, empty environment.
func toolEnv(allow []string, target buildgraph.Target) []string {
	env := append([]string{}, allow...)
	return append(env, "KIGUMI_TARGET_TRIPLE="+target.Triple, "KIGUMI_TARGET_SYS="+target.Sys)
}

func envNames(env []string) string {
	names := make([]string, 0, len(env))
	for _, kv := range env {
		names = append(names, strings.SplitN(kv, "=", 2)[0])
	}
	return strings.Join(names, ",")
}
