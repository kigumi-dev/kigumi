package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/version"
)

func TestFingerprintMismatch(t *testing.T) {
	base := driver.ArtifactFingerprint{CompilerRelease: "0.1.0", CompilerCommit: "abc123", Target: "x86_64-freestanding", AbiRevision: 1}
	if r := driver.FingerprintMismatch(base, base); len(r) != 0 {
		t.Errorf("identical fingerprints should match, got %v", r)
	}
	cases := []struct {
		name string
		got  driver.ArtifactFingerprint
	}{
		{"commit", driver.ArtifactFingerprint{CompilerRelease: "0.1.0", CompilerCommit: "def456", Target: base.Target, AbiRevision: base.AbiRevision}},
		{"target", driver.ArtifactFingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, Target: "aarch64-linux", AbiRevision: base.AbiRevision}},
		{"abi", driver.ArtifactFingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, Target: base.Target, AbiRevision: base.AbiRevision + 1}},
		{"dirty", driver.ArtifactFingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, CompilerDirty: true, Target: base.Target, AbiRevision: base.AbiRevision}},
		{"zig", driver.ArtifactFingerprint{CompilerRelease: base.CompilerRelease, CompilerCommit: base.CompilerCommit, Target: base.Target, AbiRevision: base.AbiRevision, ZigVersion: "0.17.0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if r := driver.FingerprintMismatch(c.got, base); len(r) == 0 {
				t.Errorf("expected a mismatch reason for %s", c.name)
			}
		})
	}
}

func TestFingerprintSidecarRoundTrip(t *testing.T) {
	out := filepath.Join(t.TempDir(), "lib.a")
	if _, ok, err := driver.ReadFingerprintSidecar(out); err != nil || ok {
		t.Fatalf("no sidecar yet: ok=%v err=%v", ok, err)
	}
	fp := driver.ArtifactFingerprint{CompilerRelease: "0.1.0", CompilerCommit: "deadbeef", Target: "x86_64-freestanding", AbiRevision: 1}
	if err := driver.WriteFingerprintSidecar(out, fp); err != nil {
		t.Fatal(err)
	}
	got, ok, err := driver.ReadFingerprintSidecar(out)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !reflect.DeepEqual(got, fp) {
		t.Errorf("got %+v, want %+v", got, fp)
	}
}

// TestBuildArchiveFingerprint: a bare build writes a matching sidecar, and a
// stale one from a different ABI revision is reported, never silently overwritten.
func TestBuildArchiveFingerprint(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"fp\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hello\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	target, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "fp.a")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, archive, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cc, err := driver.CCompilerFor(m)
	if err != nil {
		t.Fatal(err)
	}
	want := driver.CurrentFingerprint(target, driver.ZigVersionOf(cc))
	got, ok, err := driver.ReadFingerprintSidecar(archive)
	if err != nil || !ok {
		t.Fatalf("expected a fingerprint sidecar: ok=%v err=%v", ok, err)
	}
	if len(got.CapabilityProvision) == 0 {
		t.Error("expected the sidecar to record what the target provides")
	}
	got.CapabilityProvision, got.CapabilityReasons = nil, nil
	if !strings.HasPrefix(got.RuntimeAbi, "kgabi1:") {
		t.Errorf("the sidecar records no runtime fingerprint: %q", got.RuntimeAbi)
	}
	want.RuntimeAbi = got.RuntimeAbi
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	stale := want
	stale.AbiRevision++
	if err := driver.WriteFingerprintSidecar(archive, stale); err != nil {
		t.Fatal(err)
	}
	m2, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	buildErr.Reset()
	if ok, err := driver.BuildWith(m2, archive, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("rebuild: %v\n%s", err, buildErr.String())
	}
	if !strings.Contains(buildErr.String(), "rebuilding") || !strings.Contains(buildErr.String(), "ABI revision") {
		t.Errorf("expected a stale-fingerprint note, got:\n%s", buildErr.String())
	}
	got2, ok, err := driver.ReadFingerprintSidecar(archive)
	if err != nil || !ok {
		t.Fatalf("expected a refreshed sidecar: ok=%v err=%v", ok, err)
	}
	got2.CapabilityProvision, got2.CapabilityReasons = nil, nil
	if !reflect.DeepEqual(got2, want) {
		t.Errorf("sidecar after rebuild = %+v, want the current fingerprint %+v", got2, want)
	}

	layout, err := llgen.RuntimeLayout()
	if err != nil {
		t.Fatal(err)
	}
	grown, err := mir.ParseStdOpRegistry(mirRegistryText(t) + fmt.Sprintf("op %d ZZ_NEW primitive () -> unit\nkey zz.new\n", len(mir.Registry.Ops)))
	if err != nil {
		t.Fatal(err)
	}
	old := want
	old.AbiBase = llgen.AbiFingerprint(grown, layout, version.AbiRevision, nil)
	old.RuntimeAbi = llgen.AbiFingerprint(grown, layout, version.AbiRevision, []string{"KIGUMI_UNWIND"})
	if err := driver.WriteFingerprintSidecar(archive, old); err != nil {
		t.Fatal(err)
	}
	m3, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	buildErr.Reset()
	if ok, err := driver.BuildWith(m3, archive, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("rebuild after a registry change: %v\n%s", err, buildErr.String())
	}
	if !strings.Contains(buildErr.String(), "rebuilding") || !strings.Contains(buildErr.String(), "runtime ABI") {
		t.Errorf("a registry change was not reported as a rebuild, got:\n%s", buildErr.String())
	}
}

func TestFingerprintMismatchRuntimeAbi(t *testing.T) {
	base := driver.ArtifactFingerprint{CompilerRelease: "0.1.0", Target: "x86_64-linux", AbiBase: "kgabi1:a", RuntimeAbi: "kgabi1:b"}
	if r := driver.FingerprintMismatch(base, base); len(r) != 0 {
		t.Fatalf("equal fingerprints reported %v", r)
	}
	otherBase := base
	otherBase.AbiBase = "kgabi1:c"
	if r := driver.FingerprintMismatch(otherBase, base); len(r) != 1 || !strings.Contains(r[0], "runtime ABI") {
		t.Errorf("a changed registry or layout digest reported %v", r)
	}
	otherFlags := base
	otherFlags.RuntimeAbi = "kgabi1:d"
	if r := driver.FingerprintMismatch(otherFlags, base); len(r) != 1 || !strings.Contains(r[0], "runtime fingerprint") {
		t.Errorf("a changed runtime flag set reported %v", r)
	}
	plan := base
	plan.RuntimeAbi = ""
	if r := driver.FingerprintMismatch(otherFlags, plan); len(r) != 0 {
		t.Errorf("a plan fingerprint with no runtime flags compared them anyway: %v", r)
	}
}
