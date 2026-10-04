package driver_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/testtool"
)

// TestManifestFresh regenerates the manifest from disk fixtures (probing the VM for engine modes) and diffs it against testdata/manifest.json; UPDATE=1 rewrites it.
func TestManifestFresh(t *testing.T) {
	var fixtures []testtool.Fixture
	fixtures = append(fixtures, staticFixtures(t, "parser", "parse")...)
	fixtures = append(fixtures, staticFixtures(t, "sem", "check")...)
	fixtures = append(fixtures, staticFixtures(t, "mir", "build")...)
	fixtures = append(fixtures, staticFixtures(t, "fmt", "format")...)
	fixtures = append(fixtures, runFixtures(t)...)
	testtool.SortFixtures(fixtures)
	m := &testtool.Manifest{Schema: testtool.SchemaVersion, Reasons: testtool.Reasons, Fixtures: fixtures}

	if os.Getenv("UPDATE") == "1" {
		if err := testtool.Save("../../testdata/manifest.json", m); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := testtool.Load("../../testdata/manifest.json")
	if err != nil {
		t.Fatalf("load testdata/manifest.json: %v (run with UPDATE=1 to generate it)", err)
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

// runFixtures classifies each run fixture's engine mode by probing the VM; panics get a regex stderr matcher since only the interpreter appends a location.
func runFixtures(t *testing.T) []testtool.Fixture {
	t.Helper()
	files, err := filepath.Glob("../../testdata/run/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]testtool.Fixture, 0, len(files))
	for _, path := range files {
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
			fx.Skip = map[string]string{"interp": "static-refusal", "vm": "static-refusal"}
			out = append(out, fx)
			continue
		}
		t.Run(base, func(t *testing.T) {
			declared := fixtureEngines(path)
			root, _, wantErr, wantExit := loadRunFixture(t, path)
			m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatalf("%v", err)
			}
			t.Chdir(t.TempDir())
			var vmOut, vmErrOut bytes.Buffer
			_, vmErr := driver.RunWith(m, driver.RunOptions{Engine: "vm"}, &vmOut, &vmErrOut, []string{"main/main.kg", "one", "two"})
			fx.Requires = []string{"c-compiler"}
			switch {
			case vmErr == nil:
				fx.Mode = []string{"interp", "vm", "native"}
			case strings.Contains(vmErr.Error(), "--engine interp"):
				fx.Mode = []string{"interp", "native"}
				fx.Skip = map[string]string{"vm": "static-refusal"}
			case strings.Contains(vmErr.Error(), "kigumi build") && declared == "interp":
				fx.Mode = []string{"interp", "native"}
				fx.Skip = map[string]string{"vm": "native-only"}
			case strings.Contains(vmErr.Error(), "kigumi build"):
				fx.Mode = []string{"native"}
				fx.Skip = map[string]string{"interp": "native-only", "vm": "native-only"}
			default:
				t.Fatalf("vm: %v", vmErr)
			}
			if wantExit == 2 && wantErr != "" {
				fx.Stderr = &testtool.StderrMatch{Type: "regex", Pattern: regexp.QuoteMeta(strings.TrimSpace(wantErr))}
			}
		})
		out = append(out, fx)
	}
	return out
}
