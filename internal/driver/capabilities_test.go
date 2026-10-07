package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func loadForCapabilities(t *testing.T, main string) *driver.Analysis {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"caps\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(main), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Prog == nil {
		t.Fatal("module has diagnostics")
	}
	return a
}

func TestCapabilitiesEmptyForPlainProgram(t *testing.T) {
	a := loadForCapabilities(t, "print \"hello\"\n")
	if caps := driver.Capabilities(a); len(caps) != 0 {
		t.Errorf("Capabilities(plain program) = %v, want none", caps)
	}
}

func TestCapabilitiesAndReasonGraphNameCString(t *testing.T) {
	a := loadForCapabilities(t, "import ffi from std/ffi\n\nlet s = \"hi\"\nlet c = ffi.CString.new(&s).unwrap()\nprint \"ok\"\n")
	caps := driver.Capabilities(a)
	if len(caps) != 1 || caps[0] != "ffi.cstring" {
		t.Fatalf("Capabilities(cstring program) = %v, want [ffi.cstring]", caps)
	}
	graph := driver.CapabilityReasons(a, driver.HostTarget())
	if len(graph) != 1 || graph[0].Capability != "ffi.cstring" || graph[0].Op != "CSTRING_NEW" || len(graph[0].Path) == 0 || !graph[0].Provided || graph[0].RootKind != "entry" {
		t.Fatalf("CapabilityReasons = %+v, want one provided ffi.cstring entry reached from the entry", graph)
	}
}

func TestCapabilityReasonsNameWithheldCapabilityPerTarget(t *testing.T) {
	a := loadForCapabilities(t, "let args = host.args()\nprint \"${args.len()}\"\n")
	wasi, err := driver.ParseTarget("wasm32-wasi", "")
	if err != nil {
		t.Fatal(err)
	}
	bare := driver.HostTarget()
	bare.Sys = "none"
	for _, c := range []struct {
		name   string
		target driver.Target
		want   bool
	}{{"host", driver.HostTarget(), true}, {"wasi", wasi, true}, {"sys none", bare, false}} {
		graph := driver.CapabilityReasons(a, c.target)
		if len(graph) == 0 || graph[0].Capability != "host.args" {
			t.Fatalf("%s: CapabilityReasons = %+v, want host.args first", c.name, graph)
		}
		if graph[0].Provided != c.want || (!c.want && graph[0].Why == "") {
			t.Errorf("%s: host.args = %+v, want provided=%v with a reason when withheld", c.name, graph[0], c.want)
		}
	}
}

func TestCapabilityReasonsImplyThreadPoolNeeds(t *testing.T) {
	a := loadForCapabilities(t, "import task from std/task\n\nasync fn leaf(n: Int) -> Int {\n    n\n}\n\nlet pool = task.threadPoolExecutor(2)\nprint \"${pool.runToCompletion(leaf(1))}\"\n")
	var threads, implied bool
	for _, r := range driver.CapabilityReasons(a, driver.HostTarget()) {
		threads = threads || (r.Capability == "task.threads" && r.Op == "TASK_THREAD_POOL_EXECUTOR" && r.Provided)
		implied = implied || (r.Capability == "task.executor" && r.RequiredBy == "task.threads")
	}
	if !threads || !implied {
		t.Fatalf("want task.threads reached through the pool op and task.executor implied by it, got %+v", driver.CapabilityReasons(a, driver.HostTarget()))
	}
	if caps := driver.Capabilities(a); len(caps) != 2 || caps[0] != "task.executor" || caps[1] != "task.threads" {
		t.Errorf("Capabilities(thread pool program) = %v, want task.executor and task.threads", caps)
	}
}
