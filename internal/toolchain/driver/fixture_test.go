package driver_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/devkit/fixture"
)

// Fails loudly on a missing/stale manifest instead of treating an absent
// entry as "nothing to check".
func loadManifest(t testing.TB) *fixture.Manifest {
	t.Helper()
	m, err := fixture.Load("../../../testdata/manifest.json")
	if err != nil {
		t.Fatalf("load testdata/manifest.json: %v (regenerate with UPDATE=1 go test -run TestManifestFresh ./internal/toolchain/driver/)", err)
	}
	return m
}

// fixtureID turns a fixture path into its manifest id, which ignores the
// physical group the file lives in.
func fixtureID(path string) string {
	return fixture.IDFromPath(path)
}

// fixturePath locates a fixture by id ("run/<name>", "sem/<name>") so a test
// names the id rather than a corpus directory.
func fixturePath(t testing.TB, id string) string {
	t.Helper()
	path, err := fixture.Find("../../..", id)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// runFixtureHeadSections are a run fixture's non-source sections. Every other name must be a
// source file, so a misspelled section fails the fixture instead of writing a file nothing reads.
var runFixtureHeadSections = map[string]bool{
	"src": true, "stdout": true, "stderr": true, "exit": true, "engines": true, "secret-san": true,
	// The MIR corpus stores its expected dump here; TestLazyStdBuildMatchesEager
	// loads those fixtures through this reader for their source only.
	"expect": true,
}

// fixtureSourceSection reports whether a fixture section names source the build compiles.
func fixtureSourceSection(name string) bool {
	return strings.HasSuffix(name, ".kg") || strings.HasSuffix(name, ".c")
}

// loadRunFixture writes a testdata/run fixture's source sections under a fresh module root (`-- src --` is
// main/main.kg) and returns the expected stdout, stderr and exit code.
func loadRunFixture(t testing.TB, path string) (root, wantOut, wantErr string, wantExit int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root = t.TempDir()
	wantOut, wantErr, wantExit = "", "", 0
	name := ""
	body := ""
	flush := func() {
		if name == "" {
			return
		}
		switch name {
		case "stdout":
			wantOut = body
		case "stderr":
			wantErr = body
		case "exit":
			n, err := strconv.Atoi(strings.TrimSpace(body))
			if err != nil {
				t.Fatalf("%s: -- exit -- %q is not an integer", path, strings.TrimSpace(body))
			}
			wantExit = n
		case "engines", "secret-san", "expect":
			// Consumed elsewhere, or a MIR golden rather than a source file.
		default:
			if !runFixtureHeadSections[name] && !fixtureSourceSection(name) {
				t.Fatalf("%s: unknown fixture section %q: want -- src --, -- stdout --, -- stderr --, -- exit --, -- engines --, -- secret-san --, or a source file ending in .kg or .c", path, name)
			}
			file := name
			if file == "src" {
				file = "main/main.kg"
			}
			full := filepath.Join(root, filepath.FromSlash(file))
			os.MkdirAll(filepath.Dir(full), 0o755)
			os.WriteFile(full, []byte(body), 0o644)
		}
	}
	for _, line := range strings.SplitAfter(string(data), "\n") {
		trimmed := strings.TrimRight(line, "\n")
		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			flush()
			name = strings.TrimSuffix(strings.TrimPrefix(trimmed, "-- "), " --")
			body = ""
			continue
		}
		body += line
	}
	flush()
	return root, wantOut, wantErr, wantExit
}

// fixtureEngines reads a run fixture's optional `-- engines --` section: absent runs everywhere, "native" only under
// TestAotCorpus (a real extern(C) declaration the VM statically refuses leaves the VM tests nothing to assert).
// The section is a declaration, not free text: a value outside the run corpus's engine names is a fixture error.
func fixtureEngines(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name, engines := "", ""
	for _, line := range strings.SplitAfter(string(data), "\n") {
		trimmed := strings.TrimRight(line, "\n")
		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			name = strings.TrimSuffix(strings.TrimPrefix(trimmed, "-- "), " --")
			continue
		}
		if name == "engines" {
			engines += line
		}
	}
	switch engines = strings.TrimSpace(engines); engines {
	case "", "native":
		return engines
	}
	t.Fatalf("%s: -- engines -- = %q, want \"native\" (or omit the section)", path, engines)
	return ""
}

// noValuePlaceholder rejects an accepted program that printed the runtime's unknown-value marker:
// a value the checker let escape reaches the program's output as `<value>`.
func noValuePlaceholder(t testing.TB, out string) {
	t.Helper()
	if strings.Contains(out, "<value>") {
		t.Errorf("stdout contains the runtime's unknown-value placeholder:\n%s", out)
	}
}
