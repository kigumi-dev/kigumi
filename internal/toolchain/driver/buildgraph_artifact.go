package driver

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"kigumi/internal/toolchain/buildgraph"
	"kigumi/internal/toolchain/module"
)

// artifactOut is where an artifact lands: `<out>/<name>` for an executable,
// `<out>/<name>.a` for a library archive.
func artifactOut(outDir string, a *buildgraph.Artifact) string {
	if a.Kind == "library" {
		return filepath.Join(outDir, a.Name+".a")
	}
	return filepath.Join(outDir, a.Name)
}

// buildArtifact loads the module for the artifact's own target with the C sources and links of the artifact and of
// everything it uses, so platform files and a second target's compile fall out of LoadModule, then links it with BuildWith.
func buildArtifact(p *BuildProgram, rg *ResolvedGraph, idx int, filePath func(int) string, outDir string, opts ExecOptions, stderr io.Writer) error {
	g := rg.Graph
	a, t := g.Artifacts[idx], rg.Targets[idx]
	root, _ := p.Module.compiler.Abs(p.Module.Root)
	var sources []string
	var links []module.Link
	seen := map[int]bool{}
	var merge func(id int)
	merge = func(id int) {
		if seen[id] {
			return
		}
		seen[id] = true
		art := g.Artifacts[id]
		for _, f := range art.ExtraSources {
			sources = append(sources, filePath(f))
		}
		for _, l := range art.ExtraLinks {
			if l.Search != "" && !filepath.IsAbs(l.Search) {
				l.Search = filepath.Join(root, filepath.FromSlash(l.Search))
			}
			links = append(links, module.Link{Library: l.Library, Search: l.Search})
		}
		for _, u := range art.Uses {
			merge(u)
		}
	}
	merge(idx)
	lo := p.Module.loadOptions()
	lo.Target = t
	lo.Deny, lo.Ceiling, lo.ArtifactName = a.Deny, a.Ceiling, a.Name
	lo.RootDirs, lo.ExtraSources, lo.ExtraLinks = []string{a.Dir}, sources, links
	m, err := LoadModule(p.Module.Root, lo)
	if err != nil {
		return fmt.Errorf("artifact %s: %w", a.Name, err)
	}
	entry, err := scriptUnder(m, a.Dir)
	if err != nil {
		return fmt.Errorf("artifact %s: %w", a.Name, err)
	}
	m.Entry = entry
	m.ExplicitEntry = true
	link, err := artifactLink(rg, idx, opts.Link)
	if err != nil {
		return err
	}
	var built BuildReport
	buildOpts := BuildOptions{Target: t, CFlags: opts.CFlags, LinkFlags: a.LinkFlags, Entry: a.Entry, Reproducible: opts.Reproducible, Env: opts.Env, Link: link, SizeMin: opts.SizeMin, Report: &built}
	if a.LinkerScript != nil {
		buildOpts.LinkerScript = filePath(*a.LinkerScript)
	}
	ok, err := BuildWith(m, artifactOut(outDir, a), stderr, buildOpts)
	if err != nil {
		return fmt.Errorf("artifact %s: %w", a.Name, err)
	}
	if !ok {
		return fmt.Errorf("artifact %s did not build", a.Name)
	}
	if opts.Reports != nil {
		opts.Reports[a.Name] = built
	}
	if report := ResourceReport(m.LastAnalysis()); len(report) > 0 {
		fmt.Fprintf(stderr, "artifact %s resources:\n", a.Name)
		for _, line := range report {
			fmt.Fprintf(stderr, "  %s\n", line)
		}
	}
	return nil
}

// scriptUnder names the one script-shaped file in the package tree at dir
// (root-relative, "" for the root itself).
func scriptUnder(m *Module, dir string) (string, error) {
	var scripts []string
	for _, path := range m.Order {
		p := m.Packages[path]
		rel := m.Rel(path)
		if p.Std || p.Module != "" || !(dir == "" || rel == dir || strings.HasPrefix(rel, dir+"/")) {
			continue
		}
		for _, t := range p.Files {
			if t.HasTopLevelStatements() || t.HasExplicitMain() {
				scripts = append(scripts, t.File.Name)
			}
		}
	}
	switch len(scripts) {
	case 1:
		return scripts[0], nil
	case 0:
		return "", fmt.Errorf("no entry file under %q", dir)
	}
	return "", fmt.Errorf("several entry files under %q: %s", dir, strings.Join(scripts, ", "))
}
