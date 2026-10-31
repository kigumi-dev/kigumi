package testtool_test

import (
	"encoding/json"
	"os/exec"
	"testing"

	"kigumi/internal/devkit/testtool"
)

// TestBuildAudit is a smoke test: the counters must run cleanly over the
// real repository and find a plausible, non-empty inventory.
func TestBuildAudit(t *testing.T) {
	a, err := testtool.BuildAudit("../../..", exec.LookPath)
	if err != nil {
		t.Fatal(err)
	}
	if a.Totals.Tests == 0 {
		t.Error("found zero Test functions in the whole repository")
	}
	if a.Totals.PackagesWithGo == 0 {
		t.Error("found zero Go package directories")
	}
	if a.Fixtures.Total == 0 {
		t.Error("found zero corpus fixtures")
	}
	b, err := a.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var round map[string]any
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("audit JSON does not parse back: %v", err)
	}
}
