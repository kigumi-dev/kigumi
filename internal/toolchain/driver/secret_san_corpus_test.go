package driver_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// secretSanCorpus runs every run fixture under --secret-san too, not only those with a `-- secret-san --` section.
var secretSanCorpus = os.Getenv("KIGUMI_SECRET_SAN_CORPUS") != ""

// secretSanReport is a run fixture's `-- secret-san --` section: the report a VM run under --secret-san ends with,
// empty when that run must match the plain one.
func secretSanReport(data string) (string, bool) {
	_, body, ok := strings.Cut(data, "\n-- secret-san --\n")
	if !ok {
		return "", false
	}
	if i := strings.Index(body, "\n-- "); i >= 0 {
		body = body[:i+1]
	}
	return body, true
}

// checkSecretSanRun reruns a fixture on the VM under --secret-san: with a report it must stop there (exit 2) after a
// prefix of the plain stdout, and without one it must print exactly what the plain run printed.
func checkSecretSanRun(t *testing.T, m *driver.Module, path string, args []string, plainOut, plainErr string, plainExit int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report, has := secretSanReport(string(data))
	if !has && !secretSanCorpus {
		return
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir(), SecretSan: true}, &out, &errOut, args)
	if err != nil {
		t.Fatalf("--secret-san: %v", err)
	}
	if report == "" {
		if out.String() != plainOut || errOut.String() != plainErr || code != plainExit {
			t.Errorf("--secret-san changed the run: exit %d (plain %d)\n--- stdout ---\n%s--- stderr ---\n%s", code, plainExit, out.String(), errOut.String())
		}
		return
	}
	if code != 2 || errOut.String() != report || !strings.HasPrefix(plainOut, out.String()) {
		t.Errorf("--secret-san: exit %d, want 2 with the report\n--- stdout ---\n%s--- stderr ---\n%s--- want stderr ---\n%s", code, out.String(), errOut.String(), report)
	}
}
