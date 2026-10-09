package driver_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const planCapabilityBuild = `import build from std/build

fn configure(b: build.Builder) -> Unit! {
    let musl = b.executable("app-musl", "cmd/app", b.target("x86_64-linux-musl"))?
    let gnu = b.executable("app-gnu", "cmd/app", b.target("x86_64-linux-gnu"))?
    let wasi = b.executable("app-wasi", "cmd/app", b.target("wasm32-wasi"))?
    let tool = b.executable("tool", "cmd/tool", b.target("x86_64-linux-musl"))?
}
`

const planCapabilityApp = `import os from std/os
import task from std/task

async fn leaf() -> Int {
    1
}

fn argCount(args: os.Args) -> usize {
    args.len()
}

let args = host.args()
print "${argCount(args)}"
print "${args.len()}"
let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(leaf())}"
`

func planFor(t *testing.T) (string, driver.PlanStats) {
	t.Helper()
	root, out := t.TempDir(), t.TempDir()
	write := func(rel, body string) {
		os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
		os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644)
	}
	write("mod.kg", "Module {\n    name: \"plancap\"\n    kigumi: \"0.1\"\n}\n")
	write("build.kg", planCapabilityBuild)
	write("cmd/app/main.kg", planCapabilityApp)
	write("cmd/tool/main.kg", "print \"tool\"\n")
	std, _ := filepath.Abs("../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	g, err := p.Run(driver.HostTarget(), nil, out)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := driver.ResolveGraph(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	return driver.PlanJSONWithStats(rg, p)
}

type planCapabilities struct {
	Artifacts []struct {
		Name        string `json:"name"`
		Fingerprint struct {
			Target              string                     `json:"target"`
			CapabilityProvision []driver.CapabilityVerdict `json:"capabilityProvision"`
			CapabilityReasons   []driver.CapabilityReason  `json:"capabilityReasons"`
		} `json:"fingerprint"`
	} `json:"artifacts"`
}

func TestPlanPrintsCapabilityReasons(t *testing.T) {
	plan, _ := planFor(t)
	var got planCapabilities
	if err := json.Unmarshal([]byte(plan), &got); err != nil {
		t.Fatal(err)
	}
	type view struct {
		Artifact  string                     `json:"artifact"`
		Target    string                     `json:"target"`
		Provision []driver.CapabilityVerdict `json:"provision"`
		Reasons   []driver.CapabilityReason  `json:"reasons"`
	}
	var views []view
	for _, a := range got.Artifacts {
		views = append(views, view{a.Name, a.Fingerprint.Target, a.Fingerprint.CapabilityProvision, a.Fingerprint.CapabilityReasons})
	}
	out, err := json.MarshalIndent(views, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden := "../../testdata/plan/capability_reasons.golden.json"
	if os.Getenv("UPDATE") == "1" {
		os.MkdirAll(filepath.Dir(golden), 0o755)
		if err := os.WriteFile(golden, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (UPDATE=1 to create it)", err)
	}
	if string(want) != string(out)+"\n" {
		t.Errorf("--plan capability reasons differ from %s (UPDATE=1 to accept)\n--- got ---\n%s", golden, out)
	}
}

func TestPlanReusesAnalysisAcrossTargets(t *testing.T) {
	_, stats := planFor(t)
	if stats.Analyses != 3 || stats.Reused != 1 {
		t.Errorf("plan stats = %+v, want 3 analyses (linux, wasi, tool) and 1 reuse (the gnu twin of the musl artifact)", stats)
	}
}
