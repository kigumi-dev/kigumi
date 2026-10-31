package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func TestRunTestsHostExitFailsTheTest(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "main"), 0o755)
	os.WriteFile(filepath.Join(dir, "main", "main.kg"), []byte(`test "leaves" {
    host.exit(4)
}

test "after" {
}
`), 0o644)
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Test: true, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunTestsReportingWith(m, driver.TestRunOptions{}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(out.String(), `FAIL "leaves"`) || !strings.Contains(out.String(), `ok   "after"`) {
		t.Fatalf("code=%d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "`host.exit` is not available under `kigumi test`") {
		t.Fatalf("stderr does not explain the refusal:\n%s", errOut.String())
	}
}
