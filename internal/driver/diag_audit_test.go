package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func moduleOf(t *testing.T, name string, files map[string]string, opts driver.LoadOptions) *driver.Module {
	t.Helper()
	root := t.TempDir()
	files["mod.kg"] = "Module {\n    name: \"" + name + "\"\n    kigumi: \"0.1\"\n}\n"
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts.Compiler = testCompiler()
	opts.StdRoot, _ = filepath.Abs("../../std")
	m, err := driver.LoadModule(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const comptimeFailures = `fn boom(n: usize) -> Int {
    let xs = Array.of(1, 2)
    xs[n]
}
fn spin() -> Int {
    let mut i = 0
    for true {
        i += 1
    }
    i
}
fn rec(n: Int) -> Int {
    rec(n + 1) + 1
}
let a: Int = comptime {
    let o: Option[Int] = None
    o.unwrap()
}
let b: Int = comptime { spin() }
let c: Int = comptime { rec(1) }
let d: Int = comptime { boom(5) }
let e: Int = comptime {
    print "in comptime"
    3
}
`

func TestComptimeFailuresHaveCodesAndTheFailingLine(t *testing.T) {
	t.Parallel()
	a, err := driver.Analyze(moduleOf(t, "ct", map[string]string{"main.kg": comptimeFailures}, driver.LoadOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	out := a.Res.Render()
	for _, want := range []string{
		"error[E2240]: the `comptime` block panicked: unwrap on None",
		"error[E2241]: the `comptime` block exceeded its step limit of 200000000 steps",
		"error[E2241]: the `comptime` block exceeded its call depth limit of 3000 nested calls",
		"error[E2240]: the `comptime` block panicked: index out of range: index 5, length 2",
		"warning: the `comptime` block printed 12 bytes, which are discarded",
		"note: it failed at this line",
		"main.kg:3:5",
		"main.kg:17:5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, banned := range []string{"took too long", "too deep", "comptime: "} {
		if strings.Contains(out, banned) {
			t.Errorf("old wording %q is still printed:\n%s", banned, out)
		}
	}
}

func TestNestedModuleImportSaysItIsAModule(t *testing.T) {
	t.Parallel()
	m := moduleOf(t, "outer", map[string]string{
		"main.kg":      "import inner from outer/inner\nprint \"${inner.hi()}\"\n",
		"inner/mod.kg": "Module {\n    name: \"inner\"\n    kigumi: \"0.1\"\n}\n",
		"inner/in.kg":  "pub fn hi() -> Int { 1 }\n",
	}, driver.LoadOptions{})
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	for _, want := range []string{"error[E102]: package `outer/inner` not found", "is a separate module (it has its own mod.kg)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStrayTopLevelStatementsAreReportedOnlyOutsideTheEntry(t *testing.T) {
	t.Parallel()
	m := moduleOf(t, "stray", map[string]string{
		"main.kg":    "import sub from stray/sub\nprint \"${sub.n()}\"\n",
		"sub/sub.kg": "pub fn n() -> Int { 1 }\nprint \"sub\"\n",
	}, driver.LoadOptions{})
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if strings.Count(out, "E915") != 1 || !strings.Contains(out, "sub/sub.kg") || strings.Contains(out, "--> main.kg") {
		t.Fatalf("want one E915, in sub/sub.kg only:\n%s", out)
	}
}

func TestTestRunnerPrintsTotalsAndFailureSites(t *testing.T) {
	t.Parallel()
	m := loadReportingFixture(t)
	var out, errOut bytes.Buffer
	code, err := driver.RunTestsReportingWith(m, driver.TestRunOptions{}, &out, &errOut)
	if err != nil || code != 1 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	for _, want := range []string{"FAIL: 1 passed, 1 failed, 0 skipped", `failed main::"bad one" (main.kg:9)`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestTestRunnerFailsWhenTheFilterMatchesNothing(t *testing.T) {
	t.Parallel()
	m := loadReportingFixture(t)
	var out, errOut bytes.Buffer
	code, err := driver.RunTestsReportingWith(m, driver.TestRunOptions{Filter: "no such test"}, &out, &errOut)
	if err != nil || code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), `no test matches --filter "no such test"`) {
		t.Fatalf("code=%d err=%v stdout=%q stderr=%q", code, err, out.String(), errOut.String())
	}
}
