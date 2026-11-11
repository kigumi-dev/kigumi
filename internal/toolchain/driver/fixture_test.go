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
			wantExit, _ = strconv.Atoi(strings.TrimSpace(body))
		case "engines", "secret-san":
			// Consumed by fixtureEngines and checkSecretSanRun, not written as a file.
		default:
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
func fixtureEngines(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
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
	return strings.TrimSpace(engines)
}

// noValuePlaceholder rejects an accepted program that printed the runtime's unknown-value marker:
// a value the checker let escape reaches the program's output as `<value>`.
func noValuePlaceholder(t testing.TB, out string) {
	t.Helper()
	if strings.Contains(out, "<value>") {
		t.Errorf("stdout contains the runtime's unknown-value placeholder:\n%s", out)
	}
}
