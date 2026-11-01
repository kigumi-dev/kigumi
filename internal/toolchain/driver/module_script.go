package driver

import (
	"fmt"

	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/module"
)

// scriptManifest reads a script's leading manifest records and removes them from the tree so the checker never sees them; inside a module they belong in mod.kg.
func (m *Module) scriptManifest(hasMod bool) error {
	for _, p := range m.Packages {
		for _, t := range p.Files {
			mf, n, ds := module.ScriptManifest(t)
			if n == 0 {
				continue
			}
			if hasMod {
				return fmt.Errorf("%s: manifest records go in %s, not in a file of a module", t.File.Name, module.ManifestName)
			}
			if m.ScriptFile != "" {
				return fmt.Errorf("%s and %s both carry manifest records; only the entry may", m.ScriptFile, t.File.Name)
			}
			if mf.Kigumi == "" && len(ds) == 0 {
				ds = append(ds, diag.Errorf(diag.Location{Span: t.List(t.Root).At(0).Span()}, "a script with manifest records needs `Kigumi { version: \"...\" }` first"))
			}
			if len(ds) > 0 {
				return fmt.Errorf("%s", diag.RenderAll(t.File, ds[:1]))
			}
			t.DropRootItems(n)
			m.Manifest, m.ScriptFile = mf, t.File.Name
		}
	}
	return nil
}
