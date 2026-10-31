// Package fixture holds the corpus manifest and its validation rules.
package fixture

import (
	"encoding/json"
	"os"
)

// SchemaVersion is the manifest format this package reads and writes.
const SchemaVersion = 1

// StderrMatch says how a fixture's stderr is checked; the zero value means
// exact comparison against the fixture's own "-- stderr --" section.
type StderrMatch struct {
	Type    string `json:"type"`              // "regex" or "ignore"; "" means exact
	Pattern string `json:"pattern,omitempty"` // for "regex"
}

// Fixture is one corpus entry: a file on disk plus what must hold of it.
type Fixture struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Path     string            `json:"path"`
	Mode     []string          `json:"mode"`
	Target   []string          `json:"target,omitempty"`
	Requires []string          `json:"requires,omitempty"`
	Observes []string          `json:"observes"`
	Stderr   *StderrMatch      `json:"stderr,omitempty"`
	Skip     map[string]string `json:"skip,omitempty"` // excluded engine/target -> reason code
}

// Manifest is the full testdata/manifest.json document.
type Manifest struct {
	Schema   int               `json:"schema"`
	Reasons  map[string]string `json:"reasons"`
	Fixtures []Fixture         `json:"fixtures"`
}

// EngineUniverse lists every engine/target a fixture kind could run under;
// Mode is always a subset and the rest must appear in Skip with a reason.
var EngineUniverse = map[string][]string{
	"run":    {"vm", "native"},
	"parser": {"parse"},
	"sem":    {"check"},
	"mir":    {"build"},
	"fmt":    {"format"},
}

// Load reads and parses a manifest file.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Save writes a manifest as indented JSON with a trailing newline, stable
// across regenerations (Fixtures must already be in a deterministic order).
func Save(path string, m *Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// ByID finds a fixture by id, nil when absent.
func (m *Manifest) ByID(id string) *Fixture {
	for i := range m.Fixtures {
		if m.Fixtures[i].ID == id {
			return &m.Fixtures[i]
		}
	}
	return nil
}

// Runs reports whether a fixture's mode names the given engine/target.
func (f *Fixture) Runs(engine string) bool {
	for _, e := range f.Mode {
		if e == engine {
			return true
		}
	}
	return false
}
