package driver

import (
	"path/filepath"
	"strings"
)

// prefixMapFlag maps Cwd to "." in debug info, __FILE__ and the strings of the binary, so two checkouts of the same sources build to the same bytes.
func prefixMapFlag(o CompilerOptions) []string {
	if o.Cwd == "" {
		return nil
	}
	return []string{"-ffile-prefix-map=" + filepath.Clean(o.Cwd) + "=."}
}

func under(dir, p string) bool {
	return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
}

// pinnedInput names a compiler input for a command that runs in dir: files under the build dir are relative to it, files under Cwd stay absolute for prefixMapFlag to rewrite, anything else is relative to dir.
func pinnedInput(o CompilerOptions, dir, in string) (string, error) {
	abs, err := o.Abs(in)
	if err != nil {
		return "", err
	}
	if o.Cwd != "" && !under(dir, abs) && under(filepath.Clean(o.Cwd), abs) {
		return abs, nil
	}
	return filepath.Rel(dir, abs)
}
