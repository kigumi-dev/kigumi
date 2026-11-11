package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// TestBuildProgramExample drives every catalog build-graph example end to end:
// configure records the graph, the harness resolves and plans it, executes it,
// checks the promised artifacts exist and compares any runnable artifact's
// stdout with expect.txt.
func TestBuildProgramExample(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	examplesRoot, _ := filepath.Abs("../../../examples")
	catalog, names, err := driver.LoadExampleCatalog(examplesRoot)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, name := range names {
		meta := catalog[name]
		if meta.Oracle != "build-graph" {
			continue
		}
		seen++
		meta, root := meta, filepath.Join(examplesRoot, name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runBuildGraph(t, root, std, meta)
		})
	}
	if seen == 0 {
		t.Fatal("no build-graph example in examples/catalog.json")
	}
}

func runBuildGraph(t *testing.T, root, std string, meta driver.ExampleMeta) {
	for _, req := range meta.Requires {
		if why := missingRequirement(req); why != "" {
			t.Skipf("requires %s: %s", req, why)
		}
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("requires c-compiler: no zig or clang on PATH")
	}
	spec := meta.Build
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	request := buildconfig.HostTarget()
	if spec.Target != "" {
		request, err = buildconfig.ParseTarget(testCompiler(), spec.Target, "")
		if err != nil {
			t.Fatalf("target %q: %v", spec.Target, err)
		}
	}
	out := t.TempDir()
	g, err := p.Run(request, spec.Args, out)
	if err != nil {
		t.Fatal(err)
	}
	roots, _ := p.DescriptorRoots()
	descriptors, err := driver.LoadFrameworks(roots)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := driver.ResolveGraph(testCompiler(), g, descriptors)
	if err != nil {
		t.Fatal(err)
	}
	if len(rg.Order) == 0 {
		t.Fatalf("resolved graph has no order")
	}
	plan := driver.PlanJSON(rg, p)
	for _, want := range append([]string{"\"order\""}, spec.PlanContains...) {
		if !strings.Contains(plan, want) {
			t.Errorf("plan lacks %s:\n%s", want, plan)
		}
	}
	opts := driver.ExecOptions{ToolEnv: testToolEnv(), Env: testToolEnv()}
	for _, mode := range meta.Modes {
		if mode == "reproducible" {
			opts.Reproducible = true
		}
	}
	var stderr bytes.Buffer
	if err := driver.ExecuteGraph(p, rg, out, opts, &stderr); err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr.String())
	}
	for _, name := range spec.Produces {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("artifact %s missing: %v", name, err)
		}
	}
	if spec.Run == nil {
		return
	}
	want, err := os.ReadFile(filepath.Join(root, "expect.txt"))
	if err != nil {
		t.Fatalf("read expect.txt: %v", err)
	}
	got, err := exec.Command(filepath.Join(out, spec.Run.Path), spec.Run.Args...).Output()
	if err != nil || string(got) != string(want) {
		t.Fatalf("run %s: %q %v, want %q", spec.Run.Path, got, err, want)
	}
}

// testToolEnv is the allowlisted environment a tool step and a reproducible
// build's nested C compiler get; the driver never inherits the ambient env.
func testToolEnv() []string {
	var env []string
	for _, name := range []string{"PATH", "LANG", "LC_ALL", "LC_CTYPE", "TMPDIR", "TEMP", "TMP", "SYSTEMROOT"} {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return env
}

// TestBuildProgramRejectsInvalidGraph keeps the graph-resolution guards the
// old single-example test checked: an unregistered tool and a host-target
// library must be rejected before anything runs.
func TestBuildProgramRejectsInvalidGraph(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("requires c-compiler: no zig or clang on PATH")
	}
	std, _ := filepath.Abs("../../../std")
	examplesRoot, _ := filepath.Abs("../../../examples")
	catalog, names, err := driver.LoadExampleCatalog(examplesRoot)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range names {
		meta := catalog[name]
		if meta.Oracle != "build-graph" {
			continue
		}
		root := filepath.Join(examplesRoot, name)
		p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
		if err != nil || !ok {
			t.Fatalf("%s: load: %v %v", name, ok, err)
		}
		g, err := p.Run(buildconfig.HostTarget(), meta.Build.Args, t.TempDir())
		if err != nil {
			t.Fatalf("%s: run configure: %v", name, err)
		}
		roots, _ := p.DescriptorRoots()
		descriptors, err := driver.LoadFrameworks(roots)
		if err != nil {
			t.Fatal(err)
		}
		if len(g.ToolSteps) > 0 {
			checked++
			tool := g.ToolSteps[0].Tool
			g.ToolSteps[0].Tool = "rm"
			if _, err := driver.ResolveGraph(testCompiler(), g, descriptors); err == nil || !strings.Contains(err.Error(), "not registered") {
				t.Errorf("%s: unregistered tool should be rejected before running: %v", name, err)
			}
			g.ToolSteps[0].Tool = tool
		}
		for i := range g.Artifacts {
			if g.Artifacts[i].Kind != "library" {
				continue
			}
			checked++
			triple := g.Artifacts[i].Target.Triple
			g.Artifacts[i].Target.Triple = ""
			if _, err := driver.ResolveGraph(testCompiler(), g, descriptors); err == nil || !strings.Contains(err.Error(), "freestanding") {
				t.Errorf("%s: host-target library should be rejected: %v", name, err)
			}
			g.Artifacts[i].Target.Triple = triple
		}
	}
	if checked == 0 {
		t.Fatal("no build-graph example exercised the tool/library guards")
	}
}

// TestBuildProgramShape rejects a build.kg without the fixed entry.
func TestBuildProgramShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"shape\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "build.kg"), []byte("import build from std/build\n\nfn configure(b: build.Builder, extra: Int) -> Void! {\n    let a = b.executable(\"x\", \"\", b.request().target())?\n}\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	if _, _, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std}); err == nil || !strings.Contains(err.Error(), "configure must be") {
		t.Fatalf("expected the shape error, got %v", err)
	}
	os.WriteFile(filepath.Join(root, "build.kg"), []byte("import build from std/build\n\nlet x = 1\n\nfn configure(b: build.Builder) -> Void! {\n    let a = b.executable(\"x\", \"\", b.request().target())?\n}\n"), 0o644)
	if _, _, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std}); err == nil {
		t.Fatal("top-level statements in build.kg should be rejected")
	}
}
