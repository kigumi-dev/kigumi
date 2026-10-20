package driver

import (
	"path/filepath"
	"strings"
)

var pathValueFlags = map[string]bool{"-I": true, "-L": true, "-T": true, "-isystem": true}

var pathFileSuffixes = []string{".c", ".o", ".a", ".ld"}

// resolveFlagPaths makes the relative paths in C compiler flags absolute against Cwd, since the compile and link commands may run in another directory.
func resolveFlagPaths(o CompilerOptions, flags []string) ([]string, error) {
	out := make([]string, 0, len(flags))
	abs := func(p string) (string, error) {
		if p == "" || filepath.IsAbs(p) {
			return p, nil
		}
		return o.Abs(p)
	}
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case pathValueFlags[f] && i+1 < len(flags):
			v, err := abs(flags[i+1])
			if err != nil {
				return nil, err
			}
			out = append(out, f, v)
			i++
		case len(f) > 2 && (strings.HasPrefix(f, "-I") || strings.HasPrefix(f, "-L") || strings.HasPrefix(f, "-T")):
			v, err := abs(f[2:])
			if err != nil {
				return nil, err
			}
			out = append(out, f[:2]+v)
		case !strings.HasPrefix(f, "-") && hasPathSuffix(f):
			v, err := abs(f)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		default:
			out = append(out, f)
		}
	}
	return out, nil
}

func hasPathSuffix(f string) bool {
	for _, s := range pathFileSuffixes {
		if strings.HasSuffix(f, s) {
			return true
		}
	}
	return false
}
