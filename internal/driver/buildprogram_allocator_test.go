package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// writeAllocatorFixture lays out a module whose build.kg declares one
// executable that calls a default-allocating std fn, under dir.
func writeAllocatorFixture(t *testing.T, dir string) {
	t.Helper()
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"allocbp\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "build.kg"), []byte(strings.Join([]string{
		"import build from std/build",
		"",
		"fn configure(b: build.Builder) -> Unit! {",
		"    let a = b.executable(\"app\", \"\", b.request().target())?",
		"}",
	}, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.kg"), []byte("let xs = Array.of(1)\n"), 0o644)
}

// TestBuildProgramAllocatorNoneRejected carries the root's `--allocator
// none` into a build.kg artifact's own checker, the same way the direct
// build path (TestAllocatorNoneProfile) does.
func TestBuildProgramAllocatorNoneRejected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAllocatorFixture(t, root)
	std, _ := filepath.Abs("../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	target := driver.HostTarget()
	target.Allocator = "none"
	out := t.TempDir()
	g, err := p.Run(target, nil, out)
	if err != nil {
		t.Fatal(err)
	}
	if g.Artifacts[0].Target.Allocator != "none" {
		t.Fatalf("artifact target lost the root allocator policy: %+v", g.Artifacts[0].Target)
	}
	rg, err := driver.ResolveGraph(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !rg.Targets[0].NoDefaultAllocator() {
		t.Fatalf("resolved target lost --allocator none: %+v", rg.Targets[0])
	}
	var stderr bytes.Buffer
	if err := driver.ExecuteGraph(p, rg, out, driver.ExecOptions{}, &stderr); err == nil {
		t.Fatal("expected the build to fail: --allocator none was silently dropped for the build.kg artifact")
	}
	if !strings.Contains(stderr.String(), "needs the default allocator") {
		t.Fatalf("expected the E959 diagnostic in stderr:\n%s", stderr.String())
	}
}

// Builder.hostTarget() names the machine kigumi runs on; the root's allocator policy must survive it too, not reset to the general allocator.
func TestBuildProgramAllocatorNoneSurvivesHostTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"allocbp3\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "build.kg"), []byte(strings.Join([]string{
		"import build from std/build",
		"",
		"fn configure(b: build.Builder) -> Unit! {",
		"    let t = b.hostTarget()",
		"    let a = b.executable(\"app\", \"\", t)?",
		"}",
	}, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let xs = Array.of(1)\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	target := driver.HostTarget()
	target.Allocator = "none"
	out := t.TempDir()
	g, err := p.Run(target, nil, out)
	if err != nil {
		t.Fatal(err)
	}
	if g.Artifacts[0].Target.Allocator != "none" {
		t.Fatalf("Builder.hostTarget() lost the root allocator policy: %+v", g.Artifacts[0].Target)
	}
	rg, err := driver.ResolveGraph(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if err := driver.ExecuteGraph(p, rg, out, driver.ExecOptions{}, &stderr); err == nil {
		t.Fatal("expected the build to fail: --allocator none was silently dropped for a b.hostTarget() artifact")
	}
	if !strings.Contains(stderr.String(), "needs the default allocator") {
		t.Fatalf("expected the E959 diagnostic in stderr:\n%s", stderr.String())
	}
}

// TestBuildProgramAllocatorNoneSurvivesExplicitTarget checks that
// Builder.target(triple), which only names a different Triple, still
// carries the root's allocator policy rather than resetting it to the
// general allocator.
func TestBuildProgramAllocatorNoneSurvivesExplicitTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"allocbp2\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "build.kg"), []byte(strings.Join([]string{
		"import build from std/build",
		"",
		"fn configure(b: build.Builder) -> Unit! {",
		"    let t = b.target(\"x86_64-freestanding\")",
		"    let a = b.library(\"core\", \"\", t)?",
		"}",
	}, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let xs = Array.of(1)\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	target := driver.HostTarget()
	target.Allocator = "none"
	g, err := p.Run(target, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if g.Artifacts[0].Target.Allocator != "none" {
		t.Fatalf("Builder.target(triple) lost the root allocator policy: %+v", g.Artifacts[0].Target)
	}
}
