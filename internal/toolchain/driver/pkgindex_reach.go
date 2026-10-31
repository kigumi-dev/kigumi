package driver

import (
	"slices"
	"sort"
	"strings"
)

// rootPackages: the explicit entry, the packages under RootDirs or every own package; std is always a root and RootImports add packages by path.
func (m *Module) rootPackages() []string {
	entryPkg := ""
	if m.ExplicitEntry && m.rootDirs == nil && !m.allOwn {
		entryPkg = m.indexedEntryPackage()
	}
	var roots []string
	for _, path := range m.index.order {
		ip := m.index.pkgs[path]
		switch {
		case ip.std || m.allPackages:
			roots = append(roots, path)
		case ip.module != "":
		case m.rootDirs != nil:
			if m.underRootDirs(path) {
				roots = append(roots, path)
			}
		case entryPkg != "":
			if path == entryPkg {
				roots = append(roots, path)
			}
		default:
			roots = append(roots, path)
		}
	}
	for _, imp := range m.rootImports {
		if m.index.pkgs[imp] != nil || m.Packages[imp] != nil {
			roots = append(roots, imp)
		}
	}
	return roots
}

func (m *Module) underRootDirs(path string) bool {
	rel := m.Rel(path)
	return slices.ContainsFunc(m.rootDirs, func(dir string) bool {
		return dir == "" || rel == dir || strings.HasPrefix(rel, dir+"/")
	})
}

// indexedEntryPackage finds the own package holding m.Entry from the index alone.
func (m *Module) indexedEntryPackage() string {
	for _, path := range m.index.order {
		ip := m.index.pkgs[path]
		if ip.std || ip.module != "" {
			continue
		}
		for _, f := range ip.files {
			if f.display == m.Entry {
				return path
			}
		}
	}
	return ""
}

// importsOf lists the packages path imports: the parse when it has one (unless scanOnly), otherwise the scanned headers. Std packages are always roots, so they report none.
func (m *Module) importsOf(path string, scanOnly bool) ([]string, error) {
	if p, ok := m.Packages[path]; ok && !scanOnly {
		var out []string
		for _, t := range p.Files {
			out = append(out, importPaths(t)...)
		}
		sort.Strings(out)
		return slices.Compact(out), nil
	}
	ip := m.index.pkgs[path]
	if ip == nil || ip.std {
		return nil, nil
	}
	var out []string
	for _, f := range ip.files {
		if err := m.scan(f); err != nil {
			return nil, err
		}
		out = append(out, f.imports...)
	}
	sort.Strings(out)
	return slices.Compact(out), nil
}

// reach follows imports from the roots over the index, parsing what it reaches when asked; a path nothing indexes yet is found by a later call.
func (m *Module) reach(roots []string, parse, scanOnly bool) ([]string, error) {
	seen := map[string]bool{}
	var order []string
	queue := slices.Clone(roots)
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if seen[path] {
			continue
		}
		ip := m.index.pkgs[path]
		if _, have := m.Packages[path]; ip == nil && !have {
			continue
		}
		seen[path] = true
		order = append(order, path)
		if parse && ip != nil && !ip.parsed {
			if err := m.parsePackage(ip); err != nil {
				return nil, err
			}
		}
		imps, err := m.importsOf(path, scanOnly)
		if err != nil {
			return nil, err
		}
		queue = append(queue, imps...)
	}
	return order, nil
}

// parseReachable parses every package the roots reach; a parsed tree can import more than the scan saw, so it repeats until nothing new appears.
func (m *Module) parseReachable() error {
	for {
		before := m.idxStats.PackagesParsed
		if _, err := m.reach(m.rootPackages(), true, false); err != nil {
			return err
		}
		if m.idxStats.PackagesParsed == before {
			return nil
		}
	}
}

// ensureParsed completes a load that deferred parsing.
func (m *Module) ensureParsed() {
	if !m.deferred {
		return
	}
	m.deferred = false
	if err := m.parseReachable(); err != nil {
		m.loadErr = err
	}
	m.setOrder()
}

func (m *Module) setOrder() {
	m.Order = make([]string, 0, len(m.Packages))
	for p := range m.Packages {
		m.Order = append(m.Order, p)
	}
	sort.Strings(m.Order)
}

// EnsureParsed finishes a DeferParse load: it parses what the root set reaches and reports an I/O failure.
func (m *Module) EnsureParsed() error {
	m.ensureParsed()
	return m.loadErr
}
