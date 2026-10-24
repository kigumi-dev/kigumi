package driver

import (
	"fmt"
	"io"
)

// linked is a compiled module and the C library its build links.
type linked struct {
	m              *Module
	code           compiled
	t              Target
	link           Linkage
	cc             []string
	cflags, lflags []string
}

// compileLinked compiles m and resolves its linkage, reloading m when its Header records were translated for another C
// library; with emit and no Header record the IR cannot depend on the linkage, so it is only checked without installing zig.
func compileLinked(m *Module, stderr io.Writer, opts BuildOptions, emit bool) (linked, bool, error) {
	t := opts.Target
	if t.OS == "" {
		t = m.compiler.HostOrAmbient()
	}
	code, ok, err := compile(m, stderr, t, emitOptionsFor(t))
	if err != nil || !ok {
		return linked{code: code}, ok, err
	}
	l := linked{m: m, code: withRuntimeABI(t, code, opts.CFlags), t: t}
	if l.cflags, err = resolveFlagPaths(m.compiler, opts.CFlags); err != nil {
		return l, false, err
	}
	if l.lflags, err = resolveFlagPaths(m.compiler, opts.LinkFlags); err != nil {
		return l, false, err
	}
	if emit && len(m.headers.triples) == 0 {
		cc := m.compiler.CCompiler()
		l.t, l.link, err = resolveLinkage(m, t, opts.Link, len(cc) == 0 || isZig(cc), l.code, l.cflags, l.lflags)
		return l, err == nil, err
	}
	if err := checkLinkedRuntimeABI(m, l.cflags, l.lflags, l.code.Abi); err != nil {
		return l, false, err
	}
	if l.cc, err = CCompilerFor(m); err != nil {
		return l, false, err
	}
	if l.cc == nil {
		return l, false, fmt.Errorf("no C compiler found (need zig or clang)")
	}
	if l.t, l.link, err = resolveLinkage(m, t, opts.Link, isZig(l.cc), l.code, l.cflags, l.lflags); err != nil {
		return l, false, err
	}
	if m.headers.match(l.t) {
		return l, true, nil
	}
	r, err := reloadForHeaders(m, l.t)
	if err != nil {
		return l, false, err
	}
	l, ok, err = compileLinked(r, stderr, opts, emit)
	m.lastAnalysis = r.lastAnalysis
	return l, ok, err
}
