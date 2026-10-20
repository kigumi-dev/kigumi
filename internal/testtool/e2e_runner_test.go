package testtool

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunE2ESmoke(t *testing.T) {
	if _, _, ok := CCompiler(exec.LookPath); !ok {
		t.Skip("no C toolchain (zig or clang) on PATH")
	}
	repoRoot, _ := filepath.Abs("../..")
	work := t.TempDir()

	var out bytes.Buffer
	if err := RunE2E(E2EOptions{Lookup: exec.LookPath, RepoRoot: repoRoot, Module: "examples/hello", StdRoot: "std", WorkDir: work}, &out); err != nil {
		t.Fatalf("RunE2E: %v\n%s", err, out.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	wantPhases := []string{"compiler-build", "check-cold", "check-warm", "build-link", "run"}
	if len(lines) != len(wantPhases) {
		t.Fatalf("got %d events, want %d:\n%s", len(lines), len(wantPhases), out.String())
	}
	for i, line := range lines {
		var e E2EEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("event %d: bad JSON: %v\n%s", i, err, line)
		}
		if e.FormatVersion != E2EFormatVersion {
			t.Errorf("event %d: formatVersion = %d, want %d", i, e.FormatVersion, E2EFormatVersion)
		}
		if e.Phase != wantPhases[i] {
			t.Errorf("event %d: phase = %q, want %q", i, e.Phase, wantPhases[i])
		}
		if !e.OK {
			t.Errorf("event %d (%s): OK = false, error = %q", i, e.Phase, e.Error)
		}
		if e.DurationMS <= 0 {
			t.Errorf("event %d (%s): durationMs = %v, want > 0", i, e.Phase, e.DurationMS)
		}
	}
	if lines[3] != "" {
		var build E2EEvent
		json.Unmarshal([]byte(lines[3]), &build)
		if build.ArtifactBytes <= 0 {
			t.Errorf("build-link: artifactBytes = %d, want > 0", build.ArtifactBytes)
		}
	}
}

func TestRunE2EWithoutLookupFails(t *testing.T) {
	var out bytes.Buffer
	err := RunE2E(E2EOptions{RepoRoot: t.TempDir(), Module: "examples/hello", StdRoot: "std", WorkDir: t.TempDir()}, &out)
	if err == nil || !strings.Contains(err.Error(), "no tool search path given") {
		t.Fatalf("RunE2E with a nil Lookup = %v, want the no tool search path error", err)
	}
}
