package driver_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/testtool"
)

// TestManifestFresh regenerates the manifest from disk fixtures (probing the VM for engine modes) and diffs it against testdata/manifest.json; UPDATE=1 rewrites it.
func TestManifestFresh(t *testing.T) {
	var fixtures []testtool.Fixture
	fixtures = append(fixtures, staticFixtures(t, "parser", "parse")...)
	fixtures = append(fixtures, staticFixtures(t, "sem", "check")...)
	fixtures = append(fixtures, staticFixtures(t, "mir", "build")...)
	fixtures = append(fixtures, staticFixtures(t, "fmt", "format")...)
	run := runFixtures(t)
	t.Cleanup(func() {
		fixtures = append(fixtures, run...)
		compareManifest(t, fixtures)
	})
}

func compareManifest(t *testing.T, fixtures []testtool.Fixture) {
	if t.Failed() {
		return
	}
	testtool.SortFixtures(fixtures)
	m := &testtool.Manifest{Schema: testtool.SchemaVersion, Reasons: testtool.Reasons, Fixtures: fixtures}

	if os.Getenv("UPDATE") == "1" {
		if err := testtool.Save("../../testdata/manifest.json", m); err != nil {
			t.Error(err)
		}
		return
	}
	want, err := testtool.Load("../../testdata/manifest.json")
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
func staticFixtures(t *testing.T, kind, mode string) []testtool.Fixture {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("../../testdata", kind, "*.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	out := make([]testtool.Fixture, 0, len(files))
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".txtar")
		out = append(out, testtool.Fixture{
			ID:       kind + "/" + base,
			Kind:     kind,
			Path:     "testdata/" + kind + "/" + base + ".txtar",
			Mode:     []string{mode},
			Observes: []string{"expect"},
		})
	}
	return out
}

// runFixtures classifies each run fixture's engine mode from the VM's static verdict; the slice is complete once the parallel subtests finish.
func runFixtures(t *testing.T) []testtool.Fixture {
	t.Helper()
	files, err := filepath.Glob("../../testdata/run/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]testtool.Fixture, len(files))
	for i, path := range files {
		base := strings.TrimSuffix(filepath.Base(path), ".txtar")
		fx := testtool.Fixture{
			ID:       "run/" + base,
			Kind:     "run",
			Path:     "testdata/run/" + base + ".txtar",
			Observes: []string{"stdout", "stderr", "exit"},
		}
		if fixtureEngines(path) == "native" {
			fx.Mode = []string{"native"}
			fx.Requires = []string{"c-compiler"}
			fx.Skip = map[string]string{"vm": "static-refusal"}
			out[i] = fx
			continue
		}
		t.Run(base, func(t *testing.T) {
			t.Parallel()
			c := checkedFixture(t, path)
			fx.Requires = []string{"c-compiler"}
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
				fx.Stderr = &testtool.StderrMatch{Type: "regex", Pattern: stackOverflowStderr}
			}
			out[i] = fx
		})
	}
	return out
}

// The VM names the overflowing function after the message and native does not.
const stackOverflowStderr = `\Apanic: stack overflow( in \S+)?\n?\z`
