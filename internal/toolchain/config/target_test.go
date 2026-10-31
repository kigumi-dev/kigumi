package config_test

import (
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
)

// TestParseTargetUnverifiedABI checks that an unrecognized ABI suffix is marked Unverified rather than rejected outright or silently accepted.
func TestParseTargetUnverifiedABI(t *testing.T) {
	t.Parallel()
	got, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, "x86_64-linux-nonsense", "")
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
		got, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, triple, "")
		if err != nil {
			t.Fatalf("%s: %v", triple, err)
		}
		if got.Unverified() {
			t.Fatalf("%s must be a recognized target, not unverified", triple)
		}
	}
}
