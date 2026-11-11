package driver_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/devkit/fixture"
)

// TestManifestFresh regenerates the manifest from disk fixtures (probing the VM for engine modes) and diffs it against testdata/manifest.json; UPDATE=1 rewrites it.
func TestManifestFresh(t *testing.T) {
	var fixtures []fixture.Fixture
	fixtures = append(fixtures, staticFixtures(t, "parser")...)
	fixtures = append(fixtures, staticFixtures(t, "sem")...)
	fixtures = append(fixtures, staticFixtures(t, "mir")...)
	fixtures = append(fixtures, staticFixtures(t, "fmt")...)
	run := runFixtures(t)
	oom := oomFixtures(t)
	t.Cleanup(func() {
		fixtures = append(fixtures, run...)
		fixtures = append(fixtures, oom...)
		compareManifest(t, fixtures)
	})
}

func compareManifest(t *testing.T, fixtures []fixture.Fixture) {
	if t.Failed() {
		return
	}
	fixture.SortFixtures(fixtures)
	m := &fixture.Manifest{Schema: fixture.SchemaVersion, Reasons: fixture.Reasons, Fixtures: fixtures}

	if os.Getenv("UPDATE") == "1" {
		if err := fixture.Save("../../../testdata/manifest.json", m); err != nil {
			t.Error(err)
		}
		return
	}
	want, err := fixture.Load("../../../testdata/manifest.json")
	if err != nil {
		t.Errorf("load testdata/manifest.json: %v (run with UPDATE=1 to generate it)", err)
		return
	}
	gotJSON, _ := json.MarshalIndent(m, "", "  ")
	wantJSON, _ := json.MarshalIndent(want, "", "  ")
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("testdata/manifest.json is stale (run with UPDATE=1 to regenerate)")
	}
}

// staticFixtures lists a single-mode kind's corpus: every file passes under one named mode and nothing is ever skipped.
func staticFixtures(t *testing.T, kind string) []fixture.Fixture {
	t.Helper()
	files, err := fixture.Corpus("../../..", kind)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]fixture.Fixture, 0, len(files))
	for _, f := range files {
		out = append(out, fixture.Fixture{
			ID:    fixture.IDFromPath(f),
			Kind:  kind,
			Group: fixture.GroupFromPath(f),
			Stage: staticStage(t, kind, f),
			Path:  relPath(t, f),
			Mode:  []string{kindMode(kind)},
		})
	}
	return out
}

func kindMode(kind string) string {
	return map[string]string{"parser": "parse", "sem": "check", "mir": "build", "fmt": "format"}[kind]
}

// staticStage takes a kind's fixed stage from its manifest universe, except for
// sem, whose fixtures carry `stage: driver` and `mir: planned` in their module
// header; the manifest mirrors that header instead of a second hand-kept list.
func staticStage(t *testing.T, kind, path string) string {
	t.Helper()
	if kind != "sem" {
		return fixture.StageUniverse[kind][0]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	section := ""
	for _, line := range strings.SplitAfter(string(data), "\n") {
		trimmed := strings.TrimRight(line, "\n")
		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			section = strings.TrimSuffix(strings.TrimPrefix(trimmed, "-- "), " --")
			continue
		}
		if section != "module" {
			continue
		}
		switch strings.TrimSpace(trimmed) {
		case "stage: driver":
			return "driver"
		case "mir: planned":
			return "planned"
		}
	}
	return "check"
}

// runFixtures classifies each run fixture's engine mode from the VM's static verdict; the slice is complete once the parallel subtests finish.
func runFixtures(t *testing.T) []fixture.Fixture {
	t.Helper()
	files, err := fixture.Corpus("../../..", "run")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]fixture.Fixture, len(files))
	for i, path := range files {
		base := strings.TrimSuffix(filepath.Base(path), fixture.Ext("run"))
		fx := fixture.Fixture{
			ID:    "run/" + base,
			Kind:  "run",
			Group: fixture.GroupFromPath(path),
			Stage: "run",
			Path:  relPath(t, path),
		}
		t.Run(base, func(t *testing.T) {
			t.Parallel()
			c := checkedFixture(t, path)
			switch {
			case c.refusal == nil:
				fx.Mode = []string{"vm", "native"}
			case strings.Contains(c.refusal.Error(), "kigumi build"):
				fx.Mode = []string{"native"}
				fx.Skip = map[string]string{"vm": "native-only"}
			default:
				t.Fatalf("vm: %v", c.refusal)
			}
			if strings.TrimSpace(c.wantErr) == "panic: stack overflow" {
				fx.Stderr = &fixture.StderrMatch{Type: "regex", Pattern: stackOverflowStderr}
			}
			out[i] = fx
		})
	}
	return out
}

// oomFixtures registers every fault corpus program. A `<name>_twice` file is
// the pair for `<name>`; the pair id is derived from disk, and `covers` is
// carried forward from the checked-in manifest because which std try function a
// sweep drives is authored metadata, not something the .kg source states.
func oomFixtures(t *testing.T) []fixture.Fixture {
	t.Helper()
	files, err := fixture.Corpus("../../..", "oom")
	if err != nil {
		t.Fatal(err)
	}
	old := loadManifestUnchecked(t)
	out := make([]fixture.Fixture, 0, len(files))
	for _, path := range files {
		base := strings.TrimSuffix(filepath.Base(path), fixture.Ext("oom"))
		id := "oom/" + base
		fx := fixture.Fixture{
			ID:    id,
			Kind:  "oom",
			Group: fixture.GroupFromPath(path),
			Stage: "fault",
			Path:  relPath(t, path),
			Mode:  []string{"fault"},
		}
		// covers is authored metadata: the sweep names a std try function,
		// which the .kg source does not state, so the checked-in manifest is
		// its home and regeneration carries it forward.
		if prev := old.ByID(id); prev != nil {
			fx.Covers = prev.Covers
		}
		ext := fixture.Ext("oom")
		switch {
		case strings.HasSuffix(base, "_twice"):
			// The second-call fixture points back at its base, so the pair is
			// mutual and consistency can verify either direction.
			if _, err := os.Stat(strings.TrimSuffix(path, "_twice"+ext) + ext); err == nil {
				fx.Pair = "oom/" + strings.TrimSuffix(base, "_twice")
			}
		default:
			if _, err := os.Stat(path[:len(path)-len(ext)] + "_twice" + ext); err == nil {
				fx.Pair = "oom/" + base + "_twice"
			}
		}
		out = append(out, fx)
	}
	return out
}

// loadManifestUnchecked reads the checked-in manifest without the schema
// guard, so a regeneration can migrate an older schema while carrying forward
// the metadata that only the manifest records.
func loadManifestUnchecked(t *testing.T) *fixture.Manifest {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/manifest.json")
	if err != nil {
		return &fixture.Manifest{}
	}
	var m fixture.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("testdata/manifest.json: %v", err)
	}
	return &m
}

func relPath(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel("../../..", path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

// The VM names the overflowing function after the message and native does not.
const stackOverflowStderr = `\Apanic: stack overflow( in \S+)?\n?\z`
