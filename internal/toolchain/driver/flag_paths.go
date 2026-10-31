package driver

import (
	"path/filepath"
	"strings"
)

var pathValueFlags = map[string]bool{"-I": true, "-L": true, "-T": true, "-isystem": true, "-iquote": true, "-idirafter": true, "-isysroot": true, "-include": true, "--sysroot": true}

var joinedPathFlags = []string{"-isystem", "-iquote", "-idirafter", "-isysroot", "--sysroot=", "-I", "-L", "-T"}

var linkerPathOptions = map[string]bool{"-T": true, "--script": true, "-rpath-link": true, "--rpath-link": true, "-L": true, "--library-path": true}

var linkerPathEqualForms = []string{"--script=", "--library-path=", "--rpath-link="}

var pathFileSuffixes = []string{".c", ".o", ".a", ".ld"}

// resolveFlagPaths makes the relative paths in C compiler flags absolute against Cwd, since the compile and link commands may run in another directory.
// -rpath operands are runtime search paths (possibly $ORIGIN forms), not build inputs, so they are left as given.
func resolveFlagPaths(o CompilerOptions, flags []string) ([]string, error) {
	r := flagResolver{o: o}
	out := make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		if f != "-Xlinker" {
			r.xlinkerOpt = false
			r.rpathValue = false
		}
		if !strings.HasPrefix(f, "-Wl,") {
			r.wlOpt = false
		}
		if pathValueFlags[f] && i+1 < len(flags) {
			v, err := r.abs(flags[i+1])
			if err != nil {
				return nil, err
			}
			out = append(out, f, v)
			i++
			continue
		}
		if f == "-Xlinker" && i+1 < len(flags) {
			v, err := r.xlinkerValue(flags[i+1])
			if err != nil {
				return nil, err
			}
			out = append(out, f, v)
			i++
			continue
		}
		v, err := r.single(f)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

type flagResolver struct {
	o          CompilerOptions
	xlinkerOpt bool
	rpathValue bool
	wlOpt      bool
}

func (r *flagResolver) abs(p string) (string, error) {
	if p == "" || filepath.IsAbs(p) {
		return p, nil
	}
	return r.o.Abs(p)
}

func (r *flagResolver) xlinkerValue(v string) (string, error) {
	if r.rpathValue {
		r.rpathValue = false
		return v, nil
	}
	prevTakesPath := r.xlinkerOpt
	r.xlinkerOpt = linkerPathOptions[v]
	r.rpathValue = v == "-rpath" || v == "--rpath"
	if prevTakesPath || (!strings.HasPrefix(v, "-") && (hasPathSuffix(v) || strings.ContainsRune(v, filepath.Separator))) {
		return r.abs(v)
	}
	for _, opt := range linkerPathEqualForms {
		if strings.HasPrefix(v, opt) {
			p, err := r.abs(v[len(opt):])
			return opt + p, err
		}
	}
	return v, nil
}

func (r *flagResolver) single(f string) (string, error) {
	switch {
	case strings.HasPrefix(f, "-Wl,"):
		return r.wl(f)
	case strings.HasPrefix(f, "@") && len(f) > 1:
		v, err := r.abs(f[1:])
		return "@" + v, err
	case strings.HasPrefix(f, "-"):
		return r.joined(f)
	case hasPathSuffix(f):
		return r.abs(f)
	}
	return f, nil
}

func (r *flagResolver) joined(f string) (string, error) {
	if strings.HasPrefix(f, "-Ttext") || strings.HasPrefix(f, "-Tdata") || strings.HasPrefix(f, "-Tbss") {
		return f, nil
	}
	for _, p := range joinedPathFlags {
		if len(f) > len(p) && strings.HasPrefix(f, p) && f[len(p)] != '=' {
			v, err := r.abs(f[len(p):])
			return p + v, err
		}
	}
	return f, nil
}

func (r *flagResolver) wl(f string) (string, error) {
	parts := strings.Split(f, ",")
	if r.wlOpt && len(parts) > 1 {
		v, err := r.abs(parts[1])
		if err != nil {
			return "", err
		}
		parts[1] = v
	}
	r.wlOpt = len(parts) > 1 && linkerPathOptions[parts[len(parts)-1]]
	for i := 2; i < len(parts); i++ {
		if !linkerPathOptions[parts[i-1]] {
			continue
		}
		v, err := r.abs(parts[i])
		if err != nil {
			return "", err
		}
		parts[i] = v
	}
	for i := 1; i < len(parts); i++ {
		for _, opt := range linkerPathEqualForms {
			if strings.HasPrefix(parts[i], opt) {
				v, err := r.abs(parts[i][len(opt):])
				if err != nil {
					return "", err
				}
				parts[i] = opt + v
			}
		}
	}
	return strings.Join(parts, ","), nil
}

func hasPathSuffix(f string) bool {
	for _, s := range pathFileSuffixes {
		if strings.HasSuffix(f, s) {
			return true
		}
	}
	return false
}
