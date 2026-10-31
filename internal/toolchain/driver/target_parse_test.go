package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/policy"
)

// TestUnverifiedTargetFailsRelease checks that the unverified-ABI violation only stops a --release build; a development build still warns and succeeds.
func TestUnverifiedTargetFailsRelease(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"u\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-linux-nonsense", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target, Policy: policy.Development()})
	if err != nil {
		t.Fatal(err)
	}
	dev := policy.Evaluate(m.Violations, policy.Development())
	if dev.HasErrors() {
		t.Fatalf("development build must only warn about an unverified target: %v", dev.Render())
	}
	if len(dev.Warnings) != 1 {
		t.Fatalf("want exactly one warning, got %v", dev.Render())
	}
	rel := policy.Evaluate(m.Violations, policy.Release())
	if !rel.HasErrors() {
		t.Fatal("a release build must refuse an unverified target")
	}
}
