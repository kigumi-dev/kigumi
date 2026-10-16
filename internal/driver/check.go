package driver

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// Check runs the semantic checker over a loaded module, adding a build's E1370.
func Check(m *Module) (*sem.Result, error) {
	if m.settled != nil {
		return sem.ReplayResult(m.settled.Render, false), nil
	}
	a, err := m.analyzeForCheck()
	if err != nil {
		return nil, err
	}
	if a.Prog != nil && !a.Res.HasErrors() {
		a.checkProvision(m.target)
	}
	if !a.Res.HasErrors() {
		m.storeSettled(a, a.Res.Render())
	}
	return a.Res, nil
}

// CheckOnly runs the checker and nothing after it: no MIR, no comptime
// evaluation, so an editor gets types and diagnostics quickly and never
// starts the VM. Files with parse errors are checked as far as their
// recovered trees allow.
func CheckOnly(m *Module) (*sem.Result, error) {
	mod, err := m.semModule()
	if err != nil {
		return nil, err
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	m.recordPackages(res, false)
	return res, nil
}

// Diagnostics renders every parse diagnostic in the module, skipping
// packages an explicit entry's build scope leaves out (buildScope). A
// Header package stays in scope regardless, like Std: an unimported one
// still owes its E994 skip warning, or it would silently disappear.
func (m *Module) Diagnostics() string {
	if m.settled != nil {
		return m.settled.ParseDiags
	}
	scope := m.buildScope()
	var sb strings.Builder
	for _, p := range m.Order {
		pkg := m.Packages[p]
		if scope != nil && !pkg.Std && !pkg.Header && !scope[p] {
			continue
		}
		for _, t := range pkg.Files {
			sb.WriteString(renderTree(t))
		}
	}
	return sb.String()
}

func renderTree(t *syntax.Tree) string {
	return diagRender(t)
}

// buildScope is the entry's transitive import closure; nil (no explicit
// entry) keeps every package in scope. Std is implicit, never imported.
func (m *Module) buildScope() map[string]bool {
	m.ensureParsed()
	if !m.ExplicitEntry {
		return nil
	}
	start := m.entryPackage()
	if start == "" {
		return nil
	}
	scope := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		p, ok := m.Packages[path]
		if !ok {
			continue
		}
		for _, t := range p.Files {
			for _, imp := range importPaths(t) {
				if scope[imp] {
					continue
				}
				scope[imp] = true
				queue = append(queue, imp)
			}
		}
	}
	return scope
}

// semModule maps packages to the checker's view: std stubs form the "std"
// module, everything else the main module, and the entry file is the one
// script-shaped file of a package under cmd/, or the only script-shaped
// file of a module without cmd/. A package an explicit entry's build scope
// leaves out (a sibling cmd/* program, say) is dropped entirely.
func (m *Module) semModule() (*sem.Module, error) { return m.semModuleOf(nil) }

// semModuleOf is semModule restricted to the own packages keep accepts; std always stays.
func (m *Module) semModuleOf(keep func(path string) bool) (*sem.Module, error) {
	m.ensureParsed()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	mod := &sem.Module{Sources: m.fileStore(), Cancel: m.cancel}
	if m.target.Freestanding() {
		mod.Target = sem.Freestanding
	}
	mod.Arch = m.target.Arch
	mod.PtrBits = m.target.Layout().PtrBits
	mod.NoDefaultAllocator = m.target.NoDefaultAllocator()
	mod.RecoverableOnly = m.target.RecoverableOnly()
	mod.NoPlatform = m.target.Sys == "none"
	mod.BlockingProfile = m.target.BlockingExecutor
	mod.OS = m.target.OS
	mod.IsrContracts = mir.IsrContracts
	mod.Interrupt = m.Manifest.Interrupt
	mod.Deny = m.deny
	mod.LayerCeiling = m.ceiling
	mod.ArtifactName = m.artifactName
	if m.Entry == "" {
		m.Entry = m.defaultEntry()
	}
	scope := m.buildScope()
	for _, path := range m.Order {
		p := m.Packages[path]
		if scope != nil && !p.Std && !scope[path] {
			continue
		}
		if keep != nil && !p.Std && !keep(path) {
			continue
		}
		sp := &sem.Package{Path: p.Path, Files: p.Files, Std: p.Std, Module: p.Module}
		if p.Std {
			sp.Module = "std"
		}
		for _, t := range p.Files {
			if m.Entry != "" && t.File.Name == m.Entry {
				sp.Entry = t
				continue
			}
			// Every cmd/ package keeps its own script as an entry, so one
			// module can hold several programs (build.kg artifacts).
			if strings.HasPrefix(m.Rel(p.Path), "cmd/") {
				if !t.HasTopLevelStatements() && !t.HasExplicitMain() {
					continue
				}
				if sp.Entry != nil {
					return nil, fmt.Errorf("package %s has two script files: %s and %s", p.Path, sp.Entry.File.Name, t.File.Name)
				}
				sp.Entry = t
			}
		}
		mod.Packages = append(mod.Packages, sp)
	}
	return mod, nil
}
