package schema

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// Options narrows a Build to one declaration attribute (`--attr`) and
// names the Target/Profile the envelope records.
type Options struct {
	Attr    string
	Target  string
	Profile string
}

type OwnPackage struct {
	Path  string
	Files []*syntax.Tree
}

// ModuleScope is the subset of a loaded module Build needs: its own packages
// (never std or a dependency), in order; driver.Module satisfies this without schema importing driver.
type ModuleScope interface {
	OwnPackages() []OwnPackage
}

// Build walks m's own packages, never std or a dependency; the returned
// map looks a Declaration back up by its originating sem.EntityID.
func Build(m ModuleScope, res *sem.Result, opt Options) (*Module, map[sem.EntityID]*Declaration) {
	byEntity := map[sem.EntityID]*Declaration{}
	var scope []string
	var decls []*Declaration
	for _, p := range m.OwnPackages() {
		path := p.Path
		scope = append(scope, path)
		for _, t := range p.Files {
			for d := range t.List(t.Root).IDs() {
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
