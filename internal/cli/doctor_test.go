package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/vm"
)

// TestDoctor checks both output formats report the std root and the JSON form carries a formatVersion.
func TestDoctor(t *testing.T) {
	std, _ := filepath.Abs("../../std")

	code, out, errOut := execute(t, "--std", std, "doctor")
	if code != 0 {
		t.Fatalf("doctor: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "release:") || !strings.Contains(out, "std") {
		t.Fatalf("doctor text output missing fields:\n%s", out)
	}

	code, out, errOut = execute(t, "--std", std, "doctor", "--format", "json")
	if code != 0 {
		t.Fatalf("doctor --format json: exit %d, stderr %q", code, errOut)
	}
	var envelope struct {
		FormatVersion int `json:"formatVersion"`
		Data          struct {
			Std struct {
				Value string `json:"value"`
			} `json:"std"`
			FormatVersions struct {
				TestEvent int `json:"testEvent"`
			} `json:"formatVersions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("doctor --format json did not parse: %v\n%s", err, out)
	}
	if envelope.FormatVersion != 1 {
		t.Errorf("formatVersion = %d, want 1", envelope.FormatVersion)
	}
	if envelope.Data.Std.Value == "" {
		t.Error("doctor --format json: data.std.value is empty")
	}
	if envelope.Data.FormatVersions.TestEvent != vm.TestEventFormatVersion {
		t.Errorf("doctor's testEvent format version = %d, want vm.TestEventFormatVersion (%d)", envelope.Data.FormatVersions.TestEvent, vm.TestEventFormatVersion)
	}

	if code, errOut := executeReal(t, "--std", std, "doctor", "--format", "bogus"); code != 2 || !strings.Contains(errOut, "--format") {
		t.Fatalf("doctor --format bogus: exit %d, stderr %q", code, errOut)
	}
}
