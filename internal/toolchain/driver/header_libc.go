package driver

import (
	"fmt"
	"slices"

	"kigumi/internal/interop/cheader"
)

// headerLibc is the C library the module's Header records were translated against; the build must link that one.
type headerLibc struct {
	triples []string
	// hostOnly says why a Header record translates only against the host C library's headers.
	hostOnly string
	opts     LoadOptions
}

// headerTriple is what translate-c sees for the next Header record: the triple a build with default options links.
func (m *Module) headerTriple() string {
	if pin := m.headers.opts.headerTriple; pin != nil {
		return *pin
	}
	t, _, err := resolveLinkage(m, m.target, LinkAuto, true, compiled{}, nil, nil)
	if err != nil {
		return m.target.compileTriple()
	}
	return t.compileTriple()
}

// importHeader translates one Header record; one that fails against the static musl headers falls back to the
// host's and keeps the build on the host C library.
func (m *Module) importHeader(opts cheader.Options, path string) (cheader.Result, error) {
	opts.Target = m.headerTriple()
	res, err := cheader.Import(opts)
	if host := m.target.compileTriple(); err != nil && opts.Target != host && m.headers.opts.headerTriple == nil {
		static := opts.Target
		opts.Target = host
		if res, err = cheader.Import(opts); err == nil {
			m.headers.hostOnly = fmt.Sprintf("Header %s translates only against the host C library's headers, not %s's", path, static)
		}
	}
	if err == nil {
		m.headers.triples = append(m.headers.triples, opts.Target)
	}
	return res, err
}

func (h headerLibc) match(t Target) bool {
	want := t.compileTriple()
	return !slices.ContainsFunc(h.triples, func(s string) bool { return s != want })
}

func libcName(triple string) string {
	if triple == "" {
		return "the host C library"
	}
	return triple
}

// reloadForHeaders loads m again with its Header records translated for the C library t links.
func reloadForHeaders(m *Module, t Target) (*Module, error) {
	triple := t.compileTriple()
	if pin := m.headers.opts.headerTriple; pin != nil {
		return nil, fmt.Errorf("Header records were translated against %s's headers, but the build links %s", libcName(*pin), libcName(triple))
	}
	lo := m.headers.opts
	lo.headerTriple = &triple
	r, err := loadModule(m.Root, lo)
	if err != nil {
		return nil, err
	}
	r.Entry, r.ExplicitEntry = m.Entry, m.ExplicitEntry
	r.headers.hostOnly = m.headers.hostOnly
	r.diagsReported = m.diagsReported
	return r, nil
}
