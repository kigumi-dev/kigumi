package driver

import "strings"

// entryPackage finds the package holding m.Entry, the root-relative file
// name an explicit entry selected.
func (m *Module) entryPackage() string {
	m.ensureParsed()
	for _, path := range m.Order {
		p := m.Packages[path]
		if p.Std || p.Module != "" {
			continue
		}
		for _, t := range p.Files {
			if t.File.Name == m.Entry {
				return path
			}
		}
	}
	return ""
}

// defaultEntry names the single script-shaped file of a module that has no
// cmd/ packages, or the root package's one when other packages have scripts too, so
// only those others report a stray top-level statement; "" leaves the choice to the cmd/ rule.
func (m *Module) defaultEntry() string {
	m.ensureParsed()
	var scripts, rootScripts []string
	for _, path := range m.Order {
		p := m.Packages[path]
		if p.Std || p.Module != "" {
			continue
		}
		if strings.HasPrefix(m.Rel(path), "cmd/") {
			return ""
		}
		for _, t := range p.Files {
			if t.HasTopLevelStatements() || t.HasExplicitMain() {
				scripts = append(scripts, t.File.Name)
				if m.Rel(path) == "" {
					rootScripts = append(rootScripts, t.File.Name)
				}
			}
		}
	}
	switch {
	case len(scripts) == 1:
		return scripts[0]
	case len(rootScripts) == 1:
		return rootScripts[0]
	}
	return ""
}
