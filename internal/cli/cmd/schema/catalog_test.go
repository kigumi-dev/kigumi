package schema_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/toolchain/catalog"
)

// TestSchemaCatalog emits the std/exp catalog without a module: the same
// document doctor --catalog prints, carrying stability, audience, support and
// SPI visibility for every package.
func TestSchemaCatalog(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	code, out, errOut := clitest.Execute(t, "--std", std, "schema", "--catalog")
	if code != 0 {
		t.Fatalf("schema --catalog: exit %d, stderr %q", code, errOut)
	}
	var envelope struct {
		FormatVersion int    `json:"formatVersion"`
		Kind          string `json:"kind"`
		Data          struct {
			Packages []struct {
				Path       string `json:"path"`
				Stability  string `json:"stability"`
				Audience   string `json:"audience"`
				Support    string `json:"support"`
				Visibility string `json:"visibility"`
				SPI        []struct {
					Name    string `json:"name"`
					Purpose string `json:"purpose"`
				} `json:"spi"`
			} `json:"packages"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("catalog did not parse: %v\n%s", err, out)
	}
	if envelope.FormatVersion != catalog.FormatVersion || envelope.Kind != "catalog" {
		t.Fatalf("envelope = %d/%q, want %d/catalog", envelope.FormatVersion, envelope.Kind, catalog.FormatVersion)
	}
	byPath := map[string]int{}
	for i, p := range envelope.Data.Packages {
		byPath[p.Path] = i
		if p.Stability == "" || p.Audience == "" || p.Support == "" || p.Visibility == "" {
			t.Errorf("%s is missing a catalog field: %+v", p.Path, p)
		}
	}
	i, ok := byPath["std/json"]
	if !ok {
		t.Fatal("std/json missing")
	}
	if len(envelope.Data.Packages[i].SPI) == 0 || envelope.Data.Packages[i].SPI[0].Purpose == "" {
		t.Errorf("std/json SPI has no purpose: %+v", envelope.Data.Packages[i].SPI)
	}
	if i, ok := byPath["std/shell/coreutil"]; !ok || envelope.Data.Packages[i].Visibility != "internal" {
		t.Errorf("std/shell/coreutil visibility is not internal")
	}
}
