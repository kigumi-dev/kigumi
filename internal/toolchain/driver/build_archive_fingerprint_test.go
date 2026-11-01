package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/toolchain/artifact"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/native/link"
	"kigumi/internal/toolchain/version"
)

// TestBuildArchiveFingerprint: a bare build writes a matching sidecar, and a
// stale one from a different ABI revision is reported, never silently overwritten.
func TestBuildArchiveFingerprint(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"fp\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hello\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-freestanding", "")
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
	want := driver.CurrentFingerprint(target, link.ZigVersionOf(cc))
	got, ok, err := artifact.ReadFingerprintSidecar(archive)
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
	if err := artifact.WriteFingerprintSidecar(archive, stale); err != nil {
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
	got2, ok, err := artifact.ReadFingerprintSidecar(archive)
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
	grown, err := opdef.ParseStdOpRegistry(mirRegistryText(t) + fmt.Sprintf("op %d ZZ_NEW primitive () -> unit\nkey zz.new\n", len(opdef.Registry.Ops)))
	if err != nil {
		t.Fatal(err)
	}
	old := want
	old.AbiBase = llgen.AbiFingerprint(grown, layout, version.AbiRevision, nil)
	old.RuntimeAbi = llgen.AbiFingerprint(grown, layout, version.AbiRevision, []string{"KIGUMI_UNWIND"})
	if err := artifact.WriteFingerprintSidecar(archive, old); err != nil {
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
