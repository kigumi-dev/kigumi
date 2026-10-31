package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
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

func TestDoctorZig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake zig script needs a POSIX shell")
	}
	std, _ := filepath.Abs("../../std")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"doctorzig\"\n    kigumi: \"0.1\"\n    zig: \"0.17.0\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	writeFakeZig(t, binDir, "0.16.0-dev.2287+eb3f16db5")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KIGUMI_ZIG_HOME", t.TempDir())
	t.Chdir(dir)

	code, out, _ := execute(t, "--std", std, "doctor")
	if code != 0 {
		t.Fatalf("doctor: exit %d", code)
	}
	if !strings.Contains(out, "zig") || !strings.Contains(out, "mismatch") || !strings.Contains(out, "0.17.0") {
		t.Fatalf("doctor text output missing the zig mismatch:\n%s", out)
	}

	code, out, _ = execute(t, "--std", std, "doctor", "--format", "json")
	if code != 0 {
		t.Fatalf("doctor --format json: exit %d", code)
	}
	var envelope struct {
		Data struct {
			Zig struct {
				Status string `json:"status"`
			} `json:"zig"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("doctor --format json did not parse: %v\n%s", err, out)
	}
	if envelope.Data.Zig.Status != "mismatch" {
		t.Errorf("zig.status = %q, want \"mismatch\"", envelope.Data.Zig.Status)
	}
}

func writeFakeZig(t *testing.T, dir, version string) {
	t.Helper()
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo '" + version + "'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "zig"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
