package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ExampleMeta is one examples/ entry: what it teaches, how it is checked and
// what it needs. A missing expect.txt is a declared exclusion rather than a
// silent skip; features name the spec tags the example demonstrates.
type ExampleMeta struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Order is the tutorial reading order; directory names never decide it.
	Order int `json:"order"`
	// Level is beginner, intermediate or advanced.
	Level string `json:"level"`
	// Entry is the entry file relative to the example root; empty means main.kg.
	Entry string `json:"entry,omitempty"`
	// Oracle is run-output, test-only or build-graph.
	Oracle string `json:"oracle"`
	// Backends is the run implementations the run-output oracle checks.
	Backends []string `json:"backends"`
	// Modes are the runner modes (default plus flags such as secret-san).
	Modes []string `json:"modes,omitempty"`
	// Requires are capability codes; a missing capability skips with the code.
	Requires []string `json:"requires,omitempty"`
	// Features are canonical tags from examples/features.json.
	Features []string `json:"features"`
	// BlockedBy records unimplemented spec IDs this example does not fake.
	BlockedBy []string `json:"blocked_by,omitempty"`
	// Build carries the build-graph oracle's artifact and stdout expectations.
	Build  *BuildGraphSpec `json:"build,omitempty"`
	Reason string          `json:"reason,omitempty"`
	Shows  string          `json:"shows"`
}

// BuildGraphSpec is what a build-graph example promises beyond running: the
// artifact files it must produce, and the program to run with the output it
// must print (expect.txt holds the exact stdout).
type BuildGraphSpec struct {
	// Args are extra `-- <flags>` handed to build.kg's configure request.
	Args []string `json:"args,omitempty"`
	// Target overrides the artifact request target; empty means the host.
	Target string `json:"target,omitempty"`
	// Produces are output-directory files the graph must leave behind.
	Produces []string `json:"produces,omitempty"`
	// Run names an artifact to execute after the graph, with its arguments.
	Run *BuildRunSpec `json:"run,omitempty"`
	// PlanContains are substrings the resolved plan JSON must carry.
	PlanContains []string `json:"plan_contains,omitempty"`
}

// BuildRunSpec executes one built artifact and compares stdout with expect.txt.
type BuildRunSpec struct {
	Path string   `json:"path"`
	Args []string `json:"args,omitempty"`
}

// ValidKinds, ValidOracles, ValidLevels, ValidBackends, ValidModes and
// ValidRequires are the catalog's closed vocabularies; an unknown value is a
// catalog error, not a silent skip.
var (
	ValidKinds    = []string{"tutorial", "application", "platform", "build-graph"}
	ValidOracles  = []string{"run-output", "test-only", "build-graph"}
	ValidLevels   = []string{"beginner", "intermediate", "advanced"}
	ValidBackends = []string{"vm", "native"}
	ValidModes    = []string{"default", "secret-san", "blocking-executor", "reproducible"}
	ValidRequires = []string{"c-compiler", "filesystem", "shell", "network-loopback", "linux-x86-64"}
)

// FeatureTag is one canonical spec tag: either required (some example must
// cover it) or blocked (no example may claim it, and blocked_by names why).
type FeatureTag struct {
	Tag       string `json:"tag"`
	Shows     string `json:"shows,omitempty"`
	BlockedBy string `json:"blocked_by,omitempty"`
	Note      string `json:"note,omitempty"`
}

type featureFile struct {
	Required []FeatureTag `json:"required"`
	Blocked  []FeatureTag `json:"blocked"`
}

type exampleCatalogFile struct {
	Examples []ExampleMeta `json:"examples"`
}

// LoadExampleCatalog reads examples/catalog.json under root and cross-checks
// it against the examples/ directories: every mod.kg needs exactly one entry,
// names and orders must be unique, and the closed vocabularies must hold.
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
	orders := make(map[int]string, len(f.Examples))
	for _, e := range f.Examples {
		if _, dup := byName[e.Name]; dup {
			return nil, nil, fmt.Errorf("examples/catalog.json: duplicate entry %q", e.Name)
		}
		if err := validateMeta(e); err != nil {
			return nil, nil, err
		}
		if prev, dup := orders[e.Order]; dup {
			return nil, nil, fmt.Errorf("examples/catalog.json: %q and %q share order %d", prev, e.Name, e.Order)
		}
		orders[e.Order] = e.Name
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
	sort.Slice(order, func(i, j int) bool { return byName[order[i]].Order < byName[order[j]].Order })
	return byName, order, nil
}

// LoadExampleFeatures reads examples/features.json: the required and blocked
// canonical tags that TestExampleCoverage checks against the catalog.
func LoadExampleFeatures(root string) (map[string]FeatureTag, map[string]FeatureTag, error) {
	b, err := os.ReadFile(filepath.Join(root, "features.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("read examples/features.json: %w", err)
	}
	var f featureFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, nil, fmt.Errorf("parse examples/features.json: %w", err)
	}
	required := make(map[string]FeatureTag, len(f.Required))
	blocked := make(map[string]FeatureTag, len(f.Blocked))
	for _, t := range f.Required {
		if t.Tag == "" {
			return nil, nil, fmt.Errorf("examples/features.json: required tag with no name")
		}
		if t.BlockedBy != "" {
			return nil, nil, fmt.Errorf("examples/features.json: required tag %q must not be blocked", t.Tag)
		}
		if _, dup := required[t.Tag]; dup {
			return nil, nil, fmt.Errorf("examples/features.json: duplicate required tag %q", t.Tag)
		}
		if _, dup := blocked[t.Tag]; dup {
			return nil, nil, fmt.Errorf("examples/features.json: %q is both required and blocked", t.Tag)
		}
		required[t.Tag] = t
	}
	for _, t := range f.Blocked {
		if t.Tag == "" || t.BlockedBy == "" {
			return nil, nil, fmt.Errorf("examples/features.json: blocked tag %q needs a blocked_by ID", t.Tag)
		}
		if _, dup := blocked[t.Tag]; dup {
			return nil, nil, fmt.Errorf("examples/features.json: duplicate blocked tag %q", t.Tag)
		}
		if _, dup := required[t.Tag]; dup {
			return nil, nil, fmt.Errorf("examples/features.json: %q is both required and blocked", t.Tag)
		}
		blocked[t.Tag] = t
	}
	return required, blocked, nil
}

func validateMeta(e ExampleMeta) error {
	if e.Name == "" {
		return fmt.Errorf("examples/catalog.json: entry with no name")
	}
	if !contains(ValidKinds, e.Kind) {
		return fmt.Errorf("examples/catalog.json: %q has unknown kind %q", e.Name, e.Kind)
	}
	if !contains(ValidOracles, e.Oracle) {
		return fmt.Errorf("examples/catalog.json: %q has unknown oracle %q", e.Name, e.Oracle)
	}
	if !contains(ValidLevels, e.Level) {
		return fmt.Errorf("examples/catalog.json: %q has unknown level %q", e.Name, e.Level)
	}
	if e.Order <= 0 {
		return fmt.Errorf("examples/catalog.json: %q has no positive order", e.Name)
	}
	if len(e.Backends) == 0 {
		return fmt.Errorf("examples/catalog.json: %q has no backends", e.Name)
	}
	for _, b := range e.Backends {
		if !contains(ValidBackends, b) {
			return fmt.Errorf("examples/catalog.json: %q has unknown backend %q", e.Name, b)
		}
	}
	for _, m := range e.Modes {
		if !contains(ValidModes, m) {
			return fmt.Errorf("examples/catalog.json: %q has unknown mode %q", e.Name, m)
		}
		if m == "reproducible" && e.Oracle != "build-graph" {
			return fmt.Errorf("examples/catalog.json: %q has mode reproducible but oracle %q", e.Name, e.Oracle)
		}
		if m == "secret-san" {
			if e.Oracle != "run-output" {
				return fmt.Errorf("examples/catalog.json: %q has mode secret-san but oracle %q", e.Name, e.Oracle)
			}
			if contains(e.Backends, "native") {
				return fmt.Errorf("examples/catalog.json: %q has mode secret-san but a native backend; the shadow checker is VM-only", e.Name)
			}
		}
	}
	for _, r := range e.Requires {
		if !contains(ValidRequires, r) {
			return fmt.Errorf("examples/catalog.json: %q has unknown requirement %q", e.Name, r)
		}
	}
	if len(e.Features) == 0 {
		return fmt.Errorf("examples/catalog.json: %q has no feature tags", e.Name)
	}
	for _, f := range e.Features {
		if f == "" {
			return fmt.Errorf("examples/catalog.json: %q has an empty feature tag", e.Name)
		}
	}
	if e.Oracle == "run-output" && len(e.Backends) == 0 {
		return fmt.Errorf("examples/catalog.json: %q has run-output but no backends", e.Name)
	}
	if e.Oracle == "build-graph" && e.Build == nil {
		return fmt.Errorf("examples/catalog.json: %q has oracle build-graph but no build expectations", e.Name)
	}
	if e.Oracle != "build-graph" && e.Build != nil {
		return fmt.Errorf("examples/catalog.json: %q has build expectations but oracle %q", e.Name, e.Oracle)
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
