package schema

import (
	"kigumi/internal/driver"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// Options narrows a Build to one declaration attribute (`--attr`) and
// names the Target/Profile the envelope records.
type Options struct {
	Attr    string
	Target  string
	Profile string
}

// Build walks m's own packages, never std or a dependency; the returned
// map looks a Declaration back up by its originating sem.EntityID.
func Build(m *driver.Module, res *sem.Result, opt Options) (*Module, map[sem.EntityID]*Declaration) {
	byEntity := map[sem.EntityID]*Declaration{}
	var scope []string
	var decls []*Declaration
	for _, path := range m.Order {
		p := m.Packages[path]
		if p.Std || p.Module != "" {
			continue
		}
		scope = append(scope, path)
		for _, t := range p.Files {
			for _, d := range t.Children(t.Root) {
				decl, ent, ok := buildDeclaration(res, t, d, path)
				if !ok || (opt.Attr != "" && !hasMetadata(decl.Metadata, opt.Attr)) {
					continue
				}
				decls = append(decls, decl)
				if ent != 0 {
					byEntity[ent] = decl
				}
			}
		}
	}
	mod := &Module{Envelope: newEnvelope(opt.Target, opt.Profile, scope), Declarations: decls}
	return mod, byEntity
}

func hasMetadata(items []Metadata, name string) bool {
	for _, m := range items {
		if m.Name == name {
			return true
		}
	}
	return false
}

func originOf(t *syntax.Tree, d syntax.NodeID) Origin {
	span := t.Span(d)
	return Origin{File: t.File.Name, Line: t.File.Line(span.Start), Generated: t.File.Generated}
}
