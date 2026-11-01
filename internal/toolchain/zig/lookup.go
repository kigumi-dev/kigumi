package zig

import (
	"fmt"
	"os"
	"path/filepath"
)

// Lookup finds an executable by name; the caller decides where it searches.
type Lookup func(name string) (string, error)

// InDirs searches dirs in order like a PATH list; relative entries are skipped, as the standard PATH search refuses them.
func InDirs(dirs []string) Lookup {
	return func(name string) (string, error) {
		windows := filepath.Separator == '\\'
		names := []string{name}
		if windows {
			names = append(names, name+".exe")
		}
		for _, dir := range dirs {
			if dir == "" || !filepath.IsAbs(dir) {
				continue
			}
			for _, n := range names {
				p := filepath.Join(dir, n)
				if st, err := os.Stat(p); err == nil && !st.IsDir() && (windows || st.Mode()&0o111 != 0) {
					return p, nil
				}
			}
		}
		return "", fmt.Errorf("%s not found in the search path", name)
	}
}

func (l Lookup) find(name string) (string, error) {
	if l == nil {
		return "", fmt.Errorf("%s not found: no search path given", name)
	}
	return l(name)
}
