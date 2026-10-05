package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func loadForCapabilities(t *testing.T, main string) *driver.Analysis {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"caps\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(main), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Prog == nil {
		t.Fatal("module has diagnostics")
	}
	return a
}

func TestCapabilitiesEmptyForPlainProgram(t *testing.T) {
	a := loadForCapabilities(t, "print \"hello\"\n")
	if caps := driver.Capabilities(a); len(caps) != 0 {
		t.Errorf("Capabilities(plain program) = %v, want none", caps)
	}
}

func TestCapabilitiesAndReasonGraphNameCString(t *testing.T) {
	a := loadForCapabilities(t, "import ffi from std/ffi\n\nlet s = \"hi\"\nlet c = ffi.CString.new(&s).unwrap()\nprint \"ok\"\n")
	caps := driver.Capabilities(a)
	if len(caps) != 1 || caps[0] != "ffi.cstring" {
		t.Fatalf("Capabilities(cstring program) = %v, want [ffi.cstring]", caps)
	}
	graph := driver.CapabilityReasonGraph(a)
	if len(graph) != 1 || graph[0].Name != "ffi.cstring" || len(graph[0].Path) == 0 {
		t.Fatalf("CapabilityReasonGraph = %+v, want one ffi.cstring entry with a non-empty path", graph)
	}
}
