package driver_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func sizeOfBuild(t *testing.T, src string) int64 {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"sizeobs\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "sizeobs")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

// Never fails on a byte count; this only records what pruning an unused import saves, as JSON.
func TestHostedBinarySizeReachabilityObservation(t *testing.T) {
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	withoutJSON := sizeOfBuild(t, "print \"hello\"\n")
	withJSON := sizeOfBuild(t, "import json from std/json\nprint \"hello\"\n")

	report, err := json.Marshal(map[string]int64{
		"bytes_without_unused_json_import": withoutJSON,
		"bytes_with_unused_json_import":    withJSON,
		"delta_bytes":                      withJSON - withoutJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("hosted binary size observation: %s", report)
}
