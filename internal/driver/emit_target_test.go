package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func loadForEmit(t *testing.T, target driver.Target) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"e\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestEmitLLVMMatchesFreestandingBuild checks that --emit-llvm for a freestanding target uses kigumi_main, the same entry a real build would link.
func TestEmitLLVMMatchesFreestandingBuild(t *testing.T) {
	t.Parallel()
	target, err := driver.ParseTarget("x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	m := loadForEmit(t, target)
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	if strings.Contains(ir, "@main(") {
		t.Fatalf("freestanding IR dump must not emit hosted @main:\n%s", ir)
	}
	if !strings.Contains(ir, "@kigumi_main(") {
		t.Fatalf("freestanding IR dump must emit @kigumi_main:\n%s", ir)
	}
}

// TestEmitLLVMDefaultsToHostedMain covers the other side of the same
// mapping: a plain (hosted) target still emits the hosted entry.
func TestEmitLLVMDefaultsToHostedMain(t *testing.T) {
	t.Parallel()
	m := loadForEmit(t, driver.HostTarget())
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(ir, "@main(") {
		t.Fatalf("hosted IR dump must emit @main:\n%s", ir)
	}
}
