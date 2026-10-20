package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

// TestParseTargetUnverifiedABI checks that an unrecognized ABI suffix is marked Unverified rather than rejected outright or silently accepted.
func TestParseTargetUnverifiedABI(t *testing.T) {
	t.Parallel()
	got, err := driver.ParseTarget(testCompiler(), "x86_64-linux-nonsense", "")
	if err != nil {
		t.Fatalf("unknown ABI must build, not error: %v", err)
	}
	if !got.Unverified() {
		t.Fatal("x86_64-linux-nonsense must be marked unverified")
	}
}

// TestParseTargetKnownABI checks that recognized ABI spellings stay verified, so a normal build never gets an unexpected release-policy violation.
func TestParseTargetKnownABI(t *testing.T) {
	t.Parallel()
	for _, triple := range []string{"x86-linux-musl", "aarch64-linux-gnu", "x86_64-linux", "x86_64-windows-msvc"} {
		got, err := driver.ParseTarget(testCompiler(), triple, "")
		if err != nil {
			t.Fatalf("%s: %v", triple, err)
		}
		if got.Unverified() {
			t.Fatalf("%s must be a recognized target, not unverified", triple)
		}
	}
}

// TestUnverifiedTargetFailsRelease checks that the unverified-ABI violation only stops a --release build; a development build still warns and succeeds.
func TestUnverifiedTargetFailsRelease(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"u\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	target, err := driver.ParseTarget(testCompiler(), "x86_64-linux-nonsense", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target, Policy: driver.Development()})
	if err != nil {
		t.Fatal(err)
	}
	dev := driver.EvaluateRelease(m, driver.Development())
	if dev.HasErrors() {
		t.Fatalf("development build must only warn about an unverified target: %v", dev.Render())
	}
	if len(dev.Warnings) != 1 {
		t.Fatalf("want exactly one warning, got %v", dev.Render())
	}
	rel := driver.EvaluateRelease(m, driver.Release())
	if !rel.HasErrors() {
		t.Fatal("a release build must refuse an unverified target")
	}
}
