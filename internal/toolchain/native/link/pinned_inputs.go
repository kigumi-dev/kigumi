package link

import (
	"path/filepath"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
)

// PrefixMapFlag maps Cwd to "." in debug info, __FILE__ and the strings of the binary, so two checkouts of the same sources build to the same bytes.
func PrefixMapFlag(o buildconfig.CompilerOptions) []string {
	if o.Cwd == "" {
		return nil
	}
	return []string{"-ffile-prefix-map=" + filepath.Clean(o.Cwd) + "=."}
}

func under(dir, p string) bool {
	return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
}

// PinnedInput names a compiler input for a command that runs in dir: files under the build dir are relative to it, files under Cwd stay absolute for PrefixMapFlag to rewrite, anything else is relative to dir.
func PinnedInput(o buildconfig.CompilerOptions, dir, in string) (string, error) {
	abs, err := o.Abs(in)
	if err != nil {
		return "", err
	}
	if o.Cwd != "" && !under(dir, abs) && under(filepath.Clean(o.Cwd), abs) {
		return abs, nil
	}
	return filepath.Rel(dir, abs)
}
