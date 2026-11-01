package artifact_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kigumi/internal/toolchain/artifact"
)

func TestFingerprintMismatch(t *testing.T) {
	base := artifact.Fingerprint{CompilerRelease: "0.1.0", CompilerCommit: "abc123", Target: "x86_64-freestanding", AbiRevision: 1}
	if r := artifact.FingerprintMismatch(base, base); len(r) != 0 {
		t.Errorf("identical fingerprints should match, got %v", r)
	}
	cases := []struct {
		name string
		got  artifact.Fingerprint
	}{
		{"commit", artifact.Fingerprint{CompilerRelease: "0.1.0", CompilerCommit: "def456", Target: base.Target, AbiRevision: base.AbiRevision}},
		{"target", artifact.Fingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, Target: "aarch64-linux", AbiRevision: base.AbiRevision}},
		{"abi", artifact.Fingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, Target: base.Target, AbiRevision: base.AbiRevision + 1}},
		{"dirty", artifact.Fingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, CompilerDirty: true, Target: base.Target, AbiRevision: base.AbiRevision}},
		{"zig", artifact.Fingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, Target: base.Target, AbiRevision: base.AbiRevision, ZigVersion: "0.17.0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if r := artifact.FingerprintMismatch(c.got, base); len(r) == 0 {
				t.Errorf("expected a mismatch reason for %s", c.name)
			}
		})
	}
}

func TestFingerprintSidecarRoundTrip(t *testing.T) {
	out := filepath.Join(t.TempDir(), "lib.a")
	if _, ok, err := artifact.ReadFingerprintSidecar(out); err != nil || ok {
		t.Fatalf("no sidecar yet: ok=%v err=%v", ok, err)
	}
	fp := artifact.Fingerprint{CompilerRelease: "0.1.0", CompilerCommit: "deadbeef", Target: "x86_64-freestanding", AbiRevision: 1}
	if err := artifact.WriteFingerprintSidecar(out, fp); err != nil {
		t.Fatal(err)
	}
	got, ok, err := artifact.ReadFingerprintSidecar(out)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !reflect.DeepEqual(got, fp) {
		t.Errorf("got %+v, want %+v", got, fp)
	}
}

func TestFingerprintMismatchRuntimeAbi(t *testing.T) {
	base := artifact.Fingerprint{CompilerRelease: "0.1.0", Target: "x86_64-linux", AbiBase: "kgabi1:a", RuntimeAbi: "kgabi1:b"}
	if r := artifact.FingerprintMismatch(base, base); len(r) != 0 {
		t.Fatalf("equal fingerprints reported %v", r)
	}
	otherBase := base
	otherBase.AbiBase = "kgabi1:c"
	if r := artifact.FingerprintMismatch(otherBase, base); len(r) != 1 || !strings.Contains(r[0], "runtime ABI") {
		t.Errorf("a changed registry or layout digest reported %v", r)
	}
	otherFlags := base
	otherFlags.RuntimeAbi = "kgabi1:d"
	if r := artifact.FingerprintMismatch(otherFlags, base); len(r) != 1 || !strings.Contains(r[0], "runtime fingerprint") {
		t.Errorf("a changed runtime flag set reported %v", r)
	}
	plan := base
	plan.RuntimeAbi = ""
	if r := artifact.FingerprintMismatch(otherFlags, plan); len(r) != 0 {
		t.Errorf("a plan fingerprint with no runtime flags compared them anyway: %v", r)
	}
}
