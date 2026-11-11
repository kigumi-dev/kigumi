package catalog

import (
	"slices"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/compiler/sem"
)

// Generate reads the policy, completes each std row from the declarations and
// docs, and returns the catalog only when every std package and layer agrees
// with the sources and the resolver.
func Generate(stdRoot string) (*Catalog, error) {
	pol, err := LoadPolicy(stdRoot)
	if err != nil {
		return nil, err
	}
	facts, err := scan(stdRoot)
	if err != nil {
		return nil, err
	}
	out := &Catalog{Format: Format, FormatVersion: FormatVersion, Packages: make([]Entry, 0, len(pol.Packages))}
	seen := make(map[string]bool, len(facts))
	var problems []error
	for _, e := range pol.Packages {
		if e.IsExp() {
			out.Packages = append(out.Packages, e)
			continue
		}
		f, ok := facts[e.Path]
		if !ok {
			problems = append(problems, errors.NewErrf("catalog: %s is listed but no std package has that path", e.Path))
			continue
		}
		seen[e.Path] = true
		e.Declarations = f.declarations
		e.SPI = f.spi
		out.Packages = append(out.Packages, e)
	}
	for p := range facts {
		if !seen[p] {
			problems = append(problems, errors.NewErrf("catalog: %s has no catalog entry", p))
		}
	}
	slices.SortFunc(out.Packages, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	if err := check(out, facts); err != nil {
		problems = append(problems, err)
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return out, nil
}

// Validate re-checks c against the tree: call it on a catalog read back from
// disk so a stale copy cannot pass as current.
func Validate(stdRoot string, c *Catalog) error {
	facts, err := scan(stdRoot)
	if err != nil {
		return err
	}
	return check(c, facts)
}

// check compares the policy against the resolver and the sources.
func check(c *Catalog, facts map[string]*facts) error {
	if err := c.Validate(); err != nil {
		return err
	}
	listed := make(map[string]Entry, len(c.Packages))
	var problems []error
	for _, e := range c.Packages {
		listed[e.Path] = e
		if e.IsExp() {
			if e.Layer != LayerNone {
				problems = append(problems, errors.NewErrf("catalog: exp package %s must not name a std layer", e.Path))
			}
			if !e.HasSys(SysHosted) || e.HasSys(SysNone) || e.HasSys(SysFreestanding) {
				problems = append(problems, errors.NewErrf("catalog: exp package %s is hosted-only", e.Path))
			}
			continue
		}
		layer, ok := sem.StdLayerFor(e.Path)
		if !ok {
			problems = append(problems, errors.NewErrf("catalog: %s is not in the std layer table", e.Path))
			continue
		}
		if want := layerName(layer); e.Layer != want {
			problems = append(problems, errors.NewErrf("catalog: %s says layer %q, the resolver says %q", e.Path, e.Layer, want))
		}
		problems = append(problems, checkAvailability(e)...)
		f := facts[e.Path]
		if f == nil {
			continue
		}
		for _, s := range e.SPI {
			if !f.declared[s.Name] {
				problems = append(problems, errors.NewErrf("catalog: %s SPI %s is not a public declaration", e.Path, s.Name))
			}
		}
		for _, imp := range f.imports {
			to, ok := listed[imp]
			if !ok || imp == e.Path {
				continue
			}
			if rank(to.Layer) > rank(e.Layer) {
				problems = append(problems, errors.NewErrf("catalog: %s (%s) imports %s (%s), which its layer forbids", e.Path, e.Layer, imp, to.Layer))
			}
		}
	}
	for _, path := range sem.StdLayerPaths() {
		if _, ok := listed[path]; !ok {
			problems = append(problems, errors.NewErrf("catalog: the layer table lists %s but the catalog does not", path))
		}
	}
	return errors.Join(problems...)
}

// checkAvailability enforces what a layer means: Foundation and Allocation
// run without an OS, Platform and Tool need a hosted one.
func checkAvailability(e Entry) []error {
	var problems []error
	hosted := e.HasSys(SysHosted)
	switch e.Layer {
	case LayerPlatform, LayerTool:
		if !hosted || e.HasSys(SysNone) || e.HasSys(SysFreestanding) {
			problems = append(problems, errors.NewErrf("catalog: %s is %s and must be hosted-only", e.Path, e.Layer))
		}
	case LayerFoundation, LayerAllocation:
		if !hosted || !e.HasSys(SysNone) || !e.HasSys(SysFreestanding) {
			problems = append(problems, errors.NewErrf("catalog: %s is %s and must be available on hosted, none and freestanding", e.Path, e.Layer))
		}
	}
	return problems
}

func layerName(l sem.Layer) Layer {
	switch l {
	case sem.LayerFoundation:
		return LayerFoundation
	case sem.LayerAllocation:
		return LayerAllocation
	case sem.LayerPlatform:
		return LayerPlatform
	case sem.LayerTool:
		return LayerTool
	}
	return LayerNone
}

// rank orders layers Foundation < Allocation < Platform < Tool; an import may
// only point at an equal or lower layer.
func rank(l Layer) int {
	switch l {
	case LayerFoundation:
		return 0
	case LayerAllocation:
		return 1
	case LayerPlatform:
		return 2
	case LayerTool:
		return 3
	}
	return -1
}
