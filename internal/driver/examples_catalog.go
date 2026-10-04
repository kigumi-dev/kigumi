package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ExampleMeta is one examples/ entry's declared kind and oracle, so a
// missing expect.txt is a declared exclusion rather than a silent skip.
type ExampleMeta struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Oracle   string   `json:"oracle"`
	Backends []string `json:"backends"`
	Reason   string   `json:"reason,omitempty"`
	Shows    string   `json:"shows"`
}

// ValidKinds and ValidOracles are the catalog's closed vocabularies.
var ValidKinds = []string{"tutorial", "application", "platform", "build-graph"}
var ValidOracles = []string{"run-output", "test-only", "build-graph"}

type exampleCatalogFile struct {
	Examples []ExampleMeta `json:"examples"`
}

// LoadExampleCatalog reads examples/catalog.json under root and cross-checks
// it against the examples/ directories: every mod.kg needs exactly one entry,
// and kind/oracle must come from the closed vocabularies above.
func LoadExampleCatalog(root string) (map[string]ExampleMeta, []string, error) {
	b, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("read examples/catalog.json: %w", err)
	}
	var f exampleCatalogFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, nil, fmt.Errorf("parse examples/catalog.json: %w", err)
	}
	byName := make(map[string]ExampleMeta, len(f.Examples))
	order := make([]string, 0, len(f.Examples))
	for _, e := range f.Examples {
		if _, dup := byName[e.Name]; dup {
			return nil, nil, fmt.Errorf("examples/catalog.json: duplicate entry %q", e.Name)
		}
		if err := validateMeta(e); err != nil {
			return nil, nil, err
		}
		if _, err := os.Stat(filepath.Join(root, e.Name, "mod.kg")); err != nil {
			return nil, nil, fmt.Errorf("examples/catalog.json: %q has no examples/%s/mod.kg", e.Name, e.Name)
		}
		byName[e.Name] = e
		order = append(order, e.Name)
	}
	dirs, err := filepath.Glob(filepath.Join(root, "*", "mod.kg"))
	if err != nil {
		return nil, nil, err
	}
	for _, mod := range dirs {
		name := filepath.Base(filepath.Dir(mod))
		if _, ok := byName[name]; !ok {
			return nil, nil, fmt.Errorf("examples/%s has no entry in examples/catalog.json", name)
		}
	}
	return byName, order, nil
}

func validateMeta(e ExampleMeta) error {
	if !contains(ValidKinds, e.Kind) {
		return fmt.Errorf("examples/catalog.json: %q has unknown kind %q", e.Name, e.Kind)
	}
	if !contains(ValidOracles, e.Oracle) {
		return fmt.Errorf("examples/catalog.json: %q has unknown oracle %q", e.Name, e.Oracle)
	}
	if e.Oracle != "run-output" && e.Reason == "" {
		return fmt.Errorf("examples/catalog.json: %q has oracle %q but no reason", e.Name, e.Oracle)
	}
	if e.Shows == "" {
		return fmt.Errorf("examples/catalog.json: %q has no shows summary", e.Name)
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
