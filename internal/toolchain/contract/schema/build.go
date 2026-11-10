package schema

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
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
				if t.Kind(d) == syntax.AbiBlock {
					abi := abiContext(t, d)
					for item := range t.View(d).AsAbiBlock().Decls().IDs() {
						buildInto(&decls, byEntity, res, t, item, path, opt, abi)
					}
					continue
				}
				buildInto(&decls, byEntity, res, t, d, path, opt, nil)
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

// buildInto appends d's declaration when it is a kind the IR covers and the
// --attr filter keeps it; abi is set for a declaration nested in an ABI block.
func buildInto(decls *[]*Declaration, byEntity map[sem.EntityID]*Declaration, res *sem.Result, t *syntax.Tree, d syntax.NodeID, pkg string, opt Options, abi *ABI) {
	decl, ent, ok := buildDeclaration(res, t, d, pkg)
	if !ok || (opt.Attr != "" && !hasMetadata(decl.Metadata, opt.Attr)) {
		return
	}
	if abi != nil {
		if decl.ABI == nil {
			decl.ABI = abi
		}
		if decl.Kind == KindType && decl.Form == typeFormName(sem.FormOpaque) {
			decl.Opaque = true
		}
	}
	*decls = append(*decls, decl)
	if ent != 0 {
		byEntity[ent] = decl
	}
}

// abiContext reads an `extern(...)`/`export(...)` block's kind and first ABI
// name, the context every declaration nested in it inherits.
func abiContext(t *syntax.Tree, block syntax.NodeID) *ABI {
	blk := t.View(block).AsAbiBlock()
	abis := blk.Abis()
	if abis.Len() == 0 {
		return nil
	}
	kind := "export"
	if blk.Tok().Kind() == token.KwExtern {
		kind = "extern"
	}
	return &ABI{Block: kind, Name: abis.At(0).Tok().Text()}
}

func originOf(t *syntax.Tree, d syntax.NodeID) Origin {
	span := t.Span(d)
	return Origin{File: t.File.Name, Line: t.File.Line(span.Start), Generated: t.File.Generated}
}
