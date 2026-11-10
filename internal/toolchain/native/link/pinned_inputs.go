package link

import (
	"path/filepath"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
)

// PrefixMapFlag maps the directories whose names reach debug info, __FILE__ and
// the strings of the binary. An explicit PrefixMaps list (a reproducible build
// names the module root, the dependency cache and the build directory) replaces
// the Cwd map, which is only a plain build's way to pin its checkout. The C
// compiler takes the last match, so the list must run widest to narrowest.
func PrefixMapFlag(o buildconfig.CompilerOptions) []string {
	var flags []string
	seen := map[string]bool{}
	add := func(dir, prefix string) {
		if dir == "" {
			return
		}
		dir = filepath.Clean(dir)
		if dir == "." || seen[dir] {
			return
		}
		seen[dir] = true
		flags = append(flags, "-ffile-prefix-map="+dir+"="+prefix)
	}
	if o.PrefixMaps == nil {
		add(o.Cwd, ".")
		return flags
	}
	for _, m := range o.PrefixMaps {
		add(m.Dir, m.Prefix)
	}
	return flags
}

func under(dir, p string) bool {
	return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
}

// mapped reports whether some -ffile-prefix-map rewrites p. A mapped input
// must reach the compiler as an absolute path, or the map has nothing to
// rewrite.
func mapped(o buildconfig.CompilerOptions, p string) bool {
	if o.PrefixMaps == nil {
		return o.Cwd != "" && under(filepath.Clean(o.Cwd), p)
	}
	for _, m := range o.PrefixMaps {
		if dir := filepath.Clean(m.Dir); dir != "." && under(dir, p) {
			return true
		}
	}
	return false
}

// PinnedInput names a compiler input for a command that runs in dir: files the
// prefix maps rewrite stay absolute, files under the build dir are relative to
// it, and anything else is relative to dir.
func PinnedInput(o buildconfig.CompilerOptions, dir, in string) (string, error) {
	abs, err := o.Abs(in)
	if err != nil {
		return "", err
	}
	if !under(dir, abs) && mapped(o, abs) {
		return abs, nil
	}
	return filepath.Rel(dir, abs)
}
