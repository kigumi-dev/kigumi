package doctor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/cli/clitest"
	"kigumi/internal/toolchain/version"
)

// TestDoctor checks both output formats report the std root and the JSON form carries a formatVersion.
func TestDoctor(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")

	code, out, errOut := clitest.Execute(t, "--std", std, "doctor")
	if code != 0 {
		t.Fatalf("doctor: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "release:") || !strings.Contains(out, "std") {
		t.Fatalf("doctor text output missing fields:\n%s", out)
	}

	code, out, errOut = clitest.Execute(t, "--std", std, "doctor", "--format", "json")
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

	if code, _, errOut := clitest.Execute(t, "--std", std, "doctor", "--format", "bogus"); code != 2 || !strings.Contains(errOut, "--format") {
		t.Fatalf("doctor --format bogus: exit %d, stderr %q", code, errOut)
	}
}

func TestDoctorZig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake zig script needs a POSIX shell")
	}
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"doctorzig\"\n    kigumi: \"0.1\"\n    zig: \"0.17.0\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	writeFakeZig(t, binDir, "0.16.0-dev.2287+eb3f16db5")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KIGUMI_ZIG_HOME", t.TempDir())
	t.Chdir(dir)

	code, out, _ := clitest.Execute(t, "--std", std, "doctor")
	if code != 0 {
		t.Fatalf("doctor: exit %d", code)
	}
	if !strings.Contains(out, "zig") || !strings.Contains(out, "mismatch") || !strings.Contains(out, "0.17.0") {
		t.Fatalf("doctor text output missing the zig mismatch:\n%s", out)
	}

	code, out, _ = clitest.Execute(t, "--std", std, "doctor", "--format", "json")
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

// TestDoctorModuleRequirement checks the manifest row and diagnostic use the
// loader's at-least rule: an older requirement is satisfied, a newer one is a
// mismatch with a diagnostic, and an equal version adds nothing.
func TestDoctorModuleRequirement(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	cases := []struct {
		name       string
		kigumi     string
		status     string
		diagnostic bool
	}{
		{"equal", version.Toolchain, "ok", false},
		{"older", "0.0", "ok", false},
		{"newer", "99.0", "mismatch", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			manifest := "Module {\n    name: \"probe\"\n    kigumi: \"" + tc.kigumi + "\"\n}\n"
			if err := os.WriteFile(filepath.Join(dir, "mod.kg"), []byte(manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(dir)

			code, out, errOut := clitest.Execute(t, "--std", std, "doctor")
			if code != 0 {
				t.Fatalf("doctor: exit %d, stderr %q", code, errOut)
			}
			if !strings.Contains(out, "["+tc.status+"]") {
				t.Errorf("doctor text output missing status %q:\n%s", tc.status, out)
			}
			if got := strings.Contains(out, "diagnostic:"); got != tc.diagnostic {
				t.Errorf("doctor text diagnostic present = %v, want %v:\n%s", got, tc.diagnostic, out)
			}

			code, out, errOut = clitest.Execute(t, "--std", std, "doctor", "--format", "json")
			if code != 0 {
				t.Fatalf("doctor --format json: exit %d, stderr %q", code, errOut)
			}
			var envelope struct {
				Data struct {
					Module struct {
						Value          string `json:"value"`
						Status         string `json:"status"`
						RequiredKigumi string `json:"requiredKigumi"`
						Toolchain      string `json:"toolchain"`
					} `json:"module"`
					Diagnostics []struct {
						Code     string `json:"code"`
						Severity string `json:"severity"`
					} `json:"diagnostics"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(out), &envelope); err != nil {
				t.Fatalf("doctor --format json did not parse: %v\n%s", err, out)
			}
			if envelope.Data.Module.Status != tc.status {
				t.Errorf("module.status = %q, want %q", envelope.Data.Module.Status, tc.status)
			}
			if got := len(envelope.Data.Diagnostics); got != 0 && !tc.diagnostic {
				t.Errorf("diagnostics = %d entries, want none", got)
			}
			if tc.diagnostic {
				if len(envelope.Data.Diagnostics) != 1 || envelope.Data.Diagnostics[0].Code != "toolchain-mismatch" || envelope.Data.Diagnostics[0].Severity != "error" {
					t.Errorf("diagnostics = %+v, want one toolchain-mismatch error", envelope.Data.Diagnostics)
				}
				if envelope.Data.Module.RequiredKigumi != tc.kigumi || envelope.Data.Module.Toolchain != version.Toolchain {
					t.Errorf("module requirement = %q toolchain = %q, want %q/%q",
						envelope.Data.Module.RequiredKigumi, envelope.Data.Module.Toolchain, tc.kigumi, version.Toolchain)
				}
			}
		})
	}
}

// TestDoctorInvalidManifest checks an unreadable mod.kg becomes an error
// diagnostic instead of an unexplained "invalid" row.
func TestDoctorInvalidManifest(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"probe\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	code, out, errOut := clitest.Execute(t, "--std", std, "doctor")
	if code != 0 {
		t.Fatalf("doctor: exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "[invalid]") || !strings.Contains(out, "diagnostic:") || !strings.Contains(out, "manifest-invalid") {
		t.Fatalf("doctor text output missing the invalid-manifest diagnostic:\n%s", out)
	}
	code, out, _ = clitest.Execute(t, "--std", std, "doctor", "--format", "json")
	if code != 0 {
		t.Fatalf("doctor --format json: exit %d", code)
	}
	var envelope struct {
		Data struct {
			Diagnostics []struct {
				Code string `json:"code"`
			} `json:"diagnostics"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("doctor --format json did not parse: %v\n%s", err, out)
	}
	if len(envelope.Data.Diagnostics) != 1 || envelope.Data.Diagnostics[0].Code != "manifest-invalid" {
		t.Errorf("diagnostics = %+v, want one manifest-invalid", envelope.Data.Diagnostics)
	}
}
