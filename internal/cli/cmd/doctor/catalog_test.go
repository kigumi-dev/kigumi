package doctor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/toolchain/catalog"
)

// TestDoctorCatalog checks the report row and the catalog emitter: every std
// package is present, doc-marked SPI survives, and the JSON is versioned.
func TestDoctorCatalog(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	code, out, errOut := clitest.Execute(t, "--std", std, "doctor")
	if code != 0 {
		t.Fatalf("doctor: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "catalog") || !strings.Contains(out, "packages") {
		t.Fatalf("doctor text output missing the catalog row:\n%s", out)
	}

	var report struct {
		Data struct {
			Catalog struct {
				Value         string `json:"value"`
				Status        string `json:"status"`
				FormatVersion int    `json:"formatVersion"`
				Packages      int    `json:"packages"`
			} `json:"catalog"`
		} `json:"data"`
	}
	code, out, errOut = clitest.Execute(t, "--std", std, "doctor", "--format", "json")
	if code != 0 {
		t.Fatalf("doctor --format json: exit %d, stderr %q", code, errOut)
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("doctor report did not parse: %v\n%s", err, out)
	}
	if report.Data.Catalog.Status != "ok" || report.Data.Catalog.Packages < 59 {
		t.Errorf("catalog row = %+v, want ok with at least the 59 std packages", report.Data.Catalog)
	}
	if report.Data.Catalog.FormatVersion != catalog.FormatVersion {
		t.Errorf("catalog formatVersion = %d, want %d", report.Data.Catalog.FormatVersion, catalog.FormatVersion)
	}

	code, out, errOut = clitest.Execute(t, "--std", std, "doctor", "--catalog")
	if code != 0 {
		t.Fatalf("doctor --catalog: exit %d, stderr %q", code, errOut)
	}
	var envelope struct {
		FormatVersion int    `json:"formatVersion"`
		Kind          string `json:"kind"`
		Data          struct {
			Packages []struct {
				Path       string `json:"path"`
				Layer      string `json:"layer"`
				Support    string `json:"support"`
				Visibility string `json:"visibility"`
				SPI        []struct {
					Name string `json:"name"`
				} `json:"spi"`
			} `json:"packages"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("catalog did not parse: %v\n%s", err, out)
	}
	if envelope.FormatVersion != catalog.FormatVersion || envelope.Kind != "catalog" {
		t.Fatalf("catalog envelope = %d/%q", envelope.FormatVersion, envelope.Kind)
	}
	found := map[string]bool{}
	spi := map[string]bool{}
	for _, p := range envelope.Data.Packages {
		found[p.Path] = true
		if p.Layer == "" && !strings.HasPrefix(p.Path, "exp/") {
			t.Errorf("%s has no layer", p.Path)
		}
		for _, s := range p.SPI {
			spi[p.Path+"#"+s.Name] = true
		}
	}
	for _, path := range []string{"std/prelude", "std/json", "std/os", "std/task", "exp/ansi"} {
		if !found[path] {
			t.Errorf("%s missing from the emitted catalog", path)
		}
	}
	for _, key := range []string{"std/json#Fields", "std/json#Record.expect", "std/os#stdout", "std/task#cancellation"} {
		if !spi[key] {
			t.Errorf("SPI %s missing from the emitted catalog", key)
		}
	}
}

// TestDoctorCatalogMissing keeps a std copy without the policy file readable:
// doctor reports the gap instead of failing the whole report.
func TestDoctorCatalogMissing(t *testing.T) {
	std := t.TempDir()
	if err := os.MkdirAll(filepath.Join(std, "prelude"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := clitest.Execute(t, "--std", std, "doctor")
	if code != 0 {
		t.Fatalf("doctor: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "catalog") || !strings.Contains(out, "missing") {
		t.Fatalf("doctor text output missing the missing-catalog row:\n%s", out)
	}
}
