package driver

import (
	"strings"

	"kigumi/internal/schema"
)

// OwnPackages satisfies schema.ModuleScope: m's own packages (never std or
// a dependency), in Order.
func (m *Module) OwnPackages() []schema.OwnPackage {
	m.ensureParsed()
	var out []schema.OwnPackage
	for _, path := range m.Order {
		p := m.Packages[path]
		if p.Std || p.Module != "" {
			continue
		}
		out = append(out, schema.OwnPackage{Path: path, Files: p.Files})
	}
	return out
}

// Rel strips the module name from one of its own package paths, giving the
// directory relative to the root ("cmd/app"); other paths pass through.
func (m *Module) Rel(pkgPath string) string {
	if m.Name == "" {
		return pkgPath
	}
	if pkgPath == m.Name {
		return ""
	}
	return strings.TrimPrefix(pkgPath, m.Name+"/")
}
